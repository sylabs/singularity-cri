// Copyright (c) 2018 Sylabs, Inc. All rights reserved.
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
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sylabs/cri/pkg/index"
	"github.com/sylabs/cri/pkg/server/image"
	"github.com/sylabs/cri/pkg/server/runtime"
	"github.com/sylabs/singularity/pkg/util/user-agent"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/kubectl/util/logs"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type flags struct {
	socket     string
	storeDir   string
	streamAddr string
}

func readFlags() flags {
	var f flags
	flag.StringVar(&f.socket, "sock", "/var/run/singularity.sock", "unix socket to serve cri services")
	flag.StringVar(&f.storeDir, "store", "/var/lib/singularity", "directory to store all pulled images")
	flag.StringVar(&f.streamAddr, "stream-addr", "127.0.0.1:12345", "streaming server address")
	flag.Parse()
	return f
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	jsonReq, _ := json.Marshal(req)
	jsonResp, _ := json.Marshal(resp)
	log.Printf("%s\n\tRequest: %s\n\tResponse: %s\n\tError: %v\n\tDuration:%s\n",
		info.FullMethod, jsonReq, jsonResp, err, time.Since(start))
	return resp, err
}

func main() {
	f := readFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	// Initialize user agent strings
	useragent.InitValue("singularity", "3.0.0")

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)

	lis, err := net.Listen("unix", f.socket)
	if err != nil {
		log.Fatalf("Could not start CRI listener: %v ", err)
	}
	defer lis.Close()

	syscall.Umask(0)
	imageIndex := index.NewImageIndex()
	syImage, err := image.NewSingularityRegistry(f.storeDir, imageIndex)
	if err != nil {
		log.Printf("Could not create Singularity image service: %v", err)
		return
	}
	syRuntime, err := runtime.NewSingularityRuntime(f.streamAddr, imageIndex)
	if err != nil {
		log.Printf("Could not create Singularity runtime service: %v", err)
		return
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logGRPC))
	k8s.RegisterRuntimeServiceServer(grpcServer, syRuntime)
	k8s.RegisterImageServiceServer(grpcServer, syImage)

	log.Printf("Singularity CRI server started on %v", lis.Addr())
	go grpcServer.Serve(lis)

	<-exitCh

	grpcServer.Stop()
	log.Println("Singularity CRI service exiting...")
}
