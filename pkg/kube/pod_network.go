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

	"github.com/golang/glog"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/singularity-cri/pkg/network"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// NetworkStatus returns pod's IP address.
func (p *Pod) NetworkStatus() *k8s.PodSandboxNetworkStatus {
	if p.network == nil {
		return nil
	}
	netIP, err := p.network.GetIP()
	if err != nil {
		glog.Warningf("Could not get IP for pod %s: %v", p.id, err)
		return nil
	}
	return &k8s.PodSandboxNetworkStatus{Ip: netIP.String()}
}

// SetUpNetwork brings up network interface and configure it
// inside pod's network namespace.
func (p *Pod) SetUpNetwork(manager *network.Manager) error {
	nsPath := p.namespacePath(specs.NetworkNamespace)
	if nsPath == "" {
		return nil
	}
	networkConfig := &network.PodConfig{
		ID:           p.id,
		Namespace:    p.GetMetadata().Namespace,
		Name:         p.GetMetadata().Name,
		NsPath:       nsPath,
		PortMappings: p.GetPortMappings(),
	}
	net, err := manager.SetUpPod(networkConfig)
	if err != nil {
		return fmt.Errorf("could not set up pod's network: %v", err)
	}
	p.network = net
	return nil
}

// TearDownNetwork tears down network interface previously
// set inside pod's network namespace.
func (p *Pod) TearDownNetwork(manager *network.Manager) error {
	if p.network == nil {
		return nil
	}
	err := manager.TearDownPod(p.network)
	if err != nil {
		return fmt.Errorf("could not tear down network: %v", err)
	}
	p.network = nil
	return nil
}
