// Copyright (c) 2018-2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/golang/glog"
	"github.com/sylabs/singularity-cri/pkg/fs"
	"github.com/sylabs/singularity-cri/pkg/index"
	"github.com/sylabs/singularity-cri/pkg/server/device"
	"github.com/sylabs/singularity-cri/pkg/server/image"
	"github.com/sylabs/singularity-cri/pkg/server/runtime"
	useragent "github.com/sylabs/singularity/pkg/util/user-agent"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/kubectl/util/logs"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	k8sDP "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var (
	errGPUNotSupported = fmt.Errorf("GPU device plugin is not supported on this host")

	configPath string
	version    = "unknown"
)

func init() {
	// We want this in init so that this flag can be set even when running test binary
	// compiled from TestRunMain. Otherwise we won't be able to pass this flag to the
	// test binary b/c it won't be initialized before main() is called and we will have
	// 'flag provided but not defined' error.
	flag.StringVar(&configPath, "config", "/usr/local/etc/sycri/sycri.yaml", "path to config file")
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}

	flag.Parse()
	logs.InitLogs()
	defer logs.FlushLogs()

	config, err := parseConfig(configPath)
	if err != nil {
		glog.Errorf("Could not parse config: %v", err)
		return
	}

	// initialize user agent strings
	useragent.InitValue("singularity", "3.1.0")
	unix.Umask(0)

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, unix.SIGINT, unix.SIGTERM, unix.SIGQUIT)

	// the next defer calls will be executed in reverse order
	// each defer is specified separately to prevent weird runtime behavior when
	// defer func in not yet called but objects are already garbage collected, e.g.
	//
	//		waitShutdown := func() {
	//			cancel()
	//			criWG.Wait()
	//			dpWG.Wait()
	//		}
	//		defer waitShutdown()
	// caused segmentation fault on ubuntu 14.04 VM in circleCI
	criWG := new(sync.WaitGroup)
	defer criWG.Wait()

	dpWG := new(sync.WaitGroup)
	defer dpWG.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := startCRI(ctx, criWG, config); err != nil {
		glog.Errorf("Could not start Singularity CRI server: %v", err)
		return
	}

	dpCtx, dpCancel := context.WithCancel(ctx)
	err = startDevicePlugin(dpCtx, dpWG, config)
	devicePluginEnabled := err == nil
	if err != nil && err != errGPUNotSupported {
		glog.Errorf("Could not start Singularity device plugin: %v", err)
		return
	}

	// if device plugin is not enabled this channel will be nil
	// and select below will not be triggered
	var fsEvents <-chan fs.WatchEvent
	if devicePluginEnabled {
		watcher, err := fs.NewWatcher(k8sDP.DevicePluginPath)
		if err != nil {
			glog.Errorf("Could not create kubelet file watcher: %v", err)
			return
		}
		defer watcher.Close()
		fsEvents = watcher.Watch(ctx)
	}

	for {
		select {
		case event := <-fsEvents:
			if event.Path == k8sDP.KubeletSocket && event.Op == fs.OpCreate {
				glog.Infof("Kubelet socket was recreated, restarting device plugin")
				dpCancel()
				dpWG.Wait()

				dpCtx, dpCancel = context.WithCancel(ctx)
				dpWG = new(sync.WaitGroup)
				if err := startDevicePlugin(dpCtx, dpWG, config); err != nil {
					glog.Errorf("Could not restart Singularity device plugin: %v", err)
					return
				}
			}
		case s := <-exitCh:
			glog.Infof("Received %s signal, shutting down...", s)
			return
		}
	}

}

func startCRI(ctx context.Context, wg *sync.WaitGroup, config Config) error {
	imageIndex := index.NewImageIndex()
	syImage, err := image.NewSingularityRegistry(config.StorageDir, imageIndex)
	if err != nil {
		return fmt.Errorf("could not create Singularity image service: %v", err)
	}
	syRuntime, err := runtime.NewSingularityRuntime(
		imageIndex,
		runtime.WithStreaming(config.StreamingURL),
		runtime.WithNetwork(config.CNIBinDir, config.CNIConfDir),
		runtime.WithBaseRunDir(config.BaseRunDir),
		runtime.WithTrashDir(config.TrashDir),
	)
	if err != nil {
		return fmt.Errorf("could not create Singularity runtime service: %v", err)
	}

	lis, err := net.Listen("unix", config.ListenSocket)
	if err != nil {
		return fmt.Errorf("could not start CRI listener: %v ", err)
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logAndRecover(config.Debug)))
	k8s.RegisterRuntimeServiceServer(grpcServer, syRuntime)
	k8s.RegisterImageServiceServer(grpcServer, syImage)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer lis.Close()

		go grpcServer.Serve(lis)
		defer grpcServer.Stop()

		glog.Infof("Singularity CRI server started on %v", lis.Addr())
		<-ctx.Done()

		glog.Info("Singularity CRI service exiting...")
		if err := syRuntime.Shutdown(); err != nil {
			glog.Errorf("Error during singularity runtime service shutdown: %v", err)
		}
	}()
	return nil
}

func startDevicePlugin(ctx context.Context, wg *sync.WaitGroup, config Config) error {
	const devicePluginSocket = "singularity.sock"

	devicePlugin, err := device.NewSingularityDevicePlugin()
	if err == device.ErrUnableToLoad || err == device.ErrNoGPUs {
		glog.Warningf("GPU support is not enabled: %v", err)
		return errGPUNotSupported
	}
	if err != nil {
		return fmt.Errorf("could not create Singularity device plugin: %v", err)
	}

	cleanup := func() {
		if err := devicePlugin.Shutdown(); err != nil {
			glog.Errorf("Error during singularity device plugin shutdown: %v", err)
		}
	}

	lis, err := net.Listen("unix", k8sDP.DevicePluginPath+devicePluginSocket)
	if err != nil {
		cleanup()
		return fmt.Errorf("could not start device plugin listener: %v ", err)
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logAndRecover(config.Debug)))
	k8sDP.RegisterDevicePluginServer(grpcServer, devicePlugin)

	register := make(chan error)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer lis.Close()

		go grpcServer.Serve(lis)
		defer grpcServer.Stop()

		err := device.RegisterInKubelet(devicePluginSocket)
		if err != nil {
			cleanup()
			register <- fmt.Errorf("could not register Singularity device plugin: %v", err)
			return
		}
		close(register)

		glog.Infof("Singularity device plugin started on %v", lis.Addr())
		<-ctx.Done()

		glog.Info("Singularity device plugin exiting...")
		cleanup()
	}()
	return <-register
}

func logAndRecover(debug bool) grpc.UnaryServerInterceptor {
	return grpc.UnaryServerInterceptor(func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, e error) {
		defer func() {
			if err := recover(); err != nil {
				glog.Errorf("Caught panic in %s: %v", info.FullMethod, err)
				e = fmt.Errorf("panic: %v", err)
			}
		}()

		resp, err := handler(ctx, req)
		if debug || err != nil {
			jsonReq, _ := json.Marshal(req)
			jsonResp, _ := json.Marshal(resp)
			logFunc := glog.Infof
			if err != nil {
				logFunc = glog.Errorf
			}
			logFunc("%s\n\tRequest: %s\n\tResponse: %s\n\tError: %v", info.FullMethod, jsonReq, jsonResp, err)
		}
		return resp, err
	})
}
