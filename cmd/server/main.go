// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/sylabs/sy-cri"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/kubectl/util/logs"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func main() {
	flag.Parse()
	logs.InitLogs()
	defer logs.FlushLogs()

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)

	socketPath := "/var/run/singularity.sock"
	defer os.Remove(socketPath)

	sock, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Error listening on socket %q: %v ", socketPath, err)
	}
	defer sock.Close()

	syRuntime, err := sycri.NewSingularityRuntimeService()
	if err != nil {
		log.Printf("Could not create Singularity runtime service: %v", err)
		return
	}
	syImage := &sycri.SignularityImageService{}

	grpcServer := grpc.NewServer()
	runtimeapi.RegisterRuntimeServiceServer(grpcServer, syRuntime)
	runtimeapi.RegisterImageServiceServer(grpcServer, syImage)

	log.Printf("starting to serve on %q", socketPath)
	go grpcServer.Serve(sock)

	<-exitCh

	log.Println("singularity service exiting...")
}
