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
	"syscall"

	"github.com/golang/glog"
	"github.com/sylabs/singularity-cri/pkg/index"
	"github.com/sylabs/singularity-cri/pkg/server/device"
	"github.com/sylabs/singularity-cri/pkg/server/image"
	"github.com/sylabs/singularity-cri/pkg/server/runtime"
	useragent "github.com/sylabs/singularity/pkg/util/user-agent"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/kubectl/util/logs"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	k8sDP "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

func logGRPC(debug bool) grpc.UnaryServerInterceptor {
	return grpc.UnaryServerInterceptor(func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
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

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "/usr/local/etc/sycri/sycri.yaml", "path to config file")
	flag.Parse()

	logs.InitLogs()
	defer logs.FlushLogs()

	config, err := parseConfig(configPath)
	if err != nil {
		glog.Errorf("Could not parse config: %v", err)
		return
	}

	// Initialize user agent strings
	useragent.InitValue("singularity", "3.1.0")
	syscall.Umask(0)

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	done := make(chan struct{})
	wg := new(sync.WaitGroup)

	defer wg.Wait()
	defer close(done)

	if err := startCRI(wg, config, done); err != nil {
		glog.Errorf("Could not start Singularity CRI server: %v", err)
		return
	}

	if err := startDevicePlugin(wg, config, done); err != nil {
		glog.Errorf("Could not start Singularity device plugin: %v", err)
		return
	}

	glog.Infof("Received %s signal, shutting down...", <-exitCh)
}

func startCRI(wg *sync.WaitGroup, config Config, done chan struct{}) error {
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
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logGRPC(config.Debug)))
	k8s.RegisterRuntimeServiceServer(grpcServer, syRuntime)
	k8s.RegisterImageServiceServer(grpcServer, syImage)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer lis.Close()

		go grpcServer.Serve(lis)
		defer grpcServer.Stop()

		glog.Infof("Singularity CRI server started on %v", lis.Addr())
		<-done

		glog.Info("Singularity CRI service exiting...")
		if err := syRuntime.Shutdown(); err != nil {
			glog.Errorf("Error during singularity runtime service shutdown: %v", err)
		}
	}()
	return nil
}

func startDevicePlugin(wg *sync.WaitGroup, config Config, done chan struct{}) error {
	const devicePluginSocket = "singularity.sock"

	devicePlugin, err := device.NewSingularityDevicePlugin()
	if err == device.ErrUnableToLoad || err == device.ErrNoGPUs {
		glog.Warningf("GPU support is not enabled: %v", err)
		return nil
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
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logGRPC(config.Debug)))
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
		<-done

		glog.Info("Singularity device plugin exiting...")
		cleanup()
	}()
	return <-register
}
