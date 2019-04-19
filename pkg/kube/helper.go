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

package kube

import (
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/klog"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func writeResolvConf(path string, config *k8s.DNSConfig) error {
	if config == nil {
		return nil
	}

	klog.V(8).Infof("Creating resolv.conf file %s", path)
	resolv, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", podResolvConfPath, err)
	}
	for _, s := range config.GetServers() {
		fmt.Fprintf(resolv, "nameserver %s\n", s)
	}
	if len(config.GetSearches()) > 0 {
		fmt.Fprintf(resolv, "search %s\n", strings.Join(config.GetSearches(), " "))
	}
	for _, o := range config.GetOptions() {
		fmt.Fprintf(resolv, "options %s\n", o)
	}
	if err = resolv.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", podResolvConfPath, err)
	}
	return nil
}

func copyFile(from, to string) error {
	dest, err := os.OpenFile(to, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not create copy destination: %v", err)
	}
	defer dest.Close()

	src, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("could not open copy source: %v", err)
	}
	defer src.Close()

	_, err = io.Copy(dest, src)
	if err != nil {
		return fmt.Errorf("could not copy files: %v", err)
	}
	return nil
}
