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

package kube

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/network"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// NetworkStatus returns POD ip address.
func (p *Pod) NetworkStatus() *k8s.PodSandboxNetworkStatus {
	if p.networkConfig != nil && p.networkConfig.Setup != nil && p.namespacePath(specs.NetworkNamespace) != "" {
		if netIP, err := p.networkConfig.Setup.GetNetworkIP("", "4"); err == nil {
			return &k8s.PodSandboxNetworkStatus{Ip: netIP.String()}
		}
		if netIP, err := p.networkConfig.Setup.GetNetworkIP("", "6"); err == nil {
			return &k8s.PodSandboxNetworkStatus{Ip: netIP.String()}
		}
	}
	return nil
}

// SetUpNetwork brings up network interface and configure it
// inside POD network namespace.
func (p *Pod) SetUpNetwork(manager *network.Manager) error {
	nsPath := p.namespacePath(specs.NetworkNamespace)
	if nsPath == "" {
		return nil
	}
	p.networkConfig = &network.PodConfig{
		ID:           p.ID(),
		Namespace:    p.GetMetadata().Namespace,
		Name:         p.GetMetadata().Name,
		NsPath:       nsPath,
		PortMappings: p.GetPortMappings(),
	}
	return manager.SetUpPod(p.networkConfig)
}

// TearDownNetwork tears down network interface previously
// set inside POD network namespace.
func (p *Pod) TearDownNetwork(manager *network.Manager) error {
	if p.networkConfig == nil {
		return nil
	}
	if p.networkConfig.Setup == nil {
		return nil
	}
	err := manager.TearDownPod(p.networkConfig)
	if err != nil {
		return fmt.Errorf("could not tear down network: %v", err)
	}
	p.networkConfig.Setup = nil
	return nil
}
