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
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/sylabs/singularity-cri/pkg/index"
	"github.com/sylabs/singularity-cri/pkg/server/image"
	"github.com/sylabs/singularity-cri/pkg/server/runtime"
	"github.com/sylabs/singularity/pkg/util/user-agent"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/kubectl/util/logs"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
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
	useragent.InitValue("singularity", "3.0.0")

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	lis, err := net.Listen("unix", config.ListenSocket)
	if err != nil {
		glog.Fatalf("Could not start CRI listener: %v ", err)
	}
	defer lis.Close()

	syscall.Umask(0)
	imageIndex := index.NewImageIndex()
	syImage, err := image.NewSingularityRegistry(config.StorageDir, imageIndex)
	if err != nil {
		glog.Errorf("Could not create Singularity image service: %v", err)
		return
	}
	syRuntime, err := runtime.NewSingularityRuntime(
		imageIndex,
		runtime.WithStreaming(config.StreamingURL),
		runtime.WithNetwork(config.CNIBinDir, config.CNIConfDir),
		runtime.WithBaseRunDir(config.BaseRunDir),
		runtime.WithTrashDir(config.TrashDir),
	)
	if err != nil {
		glog.Errorf("Could not create Singularity runtime service: %v", err)
		return
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logGRPC(config.Debug)))
	k8s.RegisterRuntimeServiceServer(grpcServer, syRuntime)
	k8s.RegisterImageServiceServer(grpcServer, syImage)

	glog.Infof("Singularity CRI server started on %v", lis.Addr())
	go grpcServer.Serve(lis)

	<-exitCh

	glog.Info("Singularity CRI service exiting...")
	if err := syRuntime.Shutdown(); err != nil {
		glog.Errorf("Error during singularity runtime service shutdown: %v", err)
	}
	grpcServer.Stop()
}
