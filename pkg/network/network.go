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

package network

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/containernetworking/cni/libcni"
	"github.com/golang/glog"
	snetwork "github.com/sylabs/singularity/pkg/network"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	// CNIBinDir is the default path to CNI plugin binaries.
	CNIBinDir = "/opt/cni/bin"
	// CNIConfDir is the default path to CNI network configuration files.
	CNIConfDir = "/etc/cni/net.d"
)

// Manager contains network manager configuration and exposes
// methods to bring up and down network interface.
type Manager struct {
	sync.RWMutex
	loNetwork      *libcni.NetworkConfigList
	defaultNetwork *libcni.NetworkConfigList
	cniPath        *snetwork.CNIPath
	podCIDR        string
}

// PodConfig contains/defines pod network configuration.
type PodConfig struct {
	ID           string
	Namespace    string
	Name         string
	NsPath       string
	PortMappings []*k8s.PortMapping
}

// PodNetwork represents set up pod's network. It is a caller's responsibility
// to tear this network down by calling Manager.TearDownPod during pod's shutdown.
// PodNetwork is also used to retrieve pod's IP address.
type PodNetwork struct {
	setup          *snetwork.Setup
	defaultNetwork string
}

// Init initializes CNI network manager.
func (m *Manager) Init(cniPath *snetwork.CNIPath) error {
	if m.cniPath != nil {
		return nil
	}
	if cniPath == nil {
		m.cniPath = &snetwork.CNIPath{
			Plugin: CNIBinDir,
			Conf:   CNIConfDir,
		}
	} else {
		m.cniPath = cniPath
	}

	return m.setDefaultNetwork()
}

// checkInit updates CNI network configuration and does some sanity checks.
func (m *Manager) checkInit() error {
	if err := m.setDefaultNetwork(); err != nil {
		return err
	}

	m.Lock()
	defer m.Unlock()

	ipRanges := false
	for _, plugin := range m.defaultNetwork.Plugins {
		if plugin.Network.Capabilities["ipRanges"] {
			if m.podCIDR == "" {
				return fmt.Errorf("no PodCIDR set")
			}
			ipRanges = true
			break
		}
	}
	if !ipRanges && m.podCIDR != "" {
		glog.V(1).Infof("Resetting pod CIDR, network plugin doesn't support it")
		m.podCIDR = ""
	}
	return nil
}

func (m *Manager) setDefaultNetwork() error {
	m.Lock()
	defer m.Unlock()

	if m.defaultNetwork != nil {
		return nil
	}
	netConfList, err := snetwork.GetAllNetworkConfigList(m.cniPath)
	if err != nil {
		return fmt.Errorf("could not get networks: %v", err)
	}
	if len(netConfList) == 0 {
		return fmt.Errorf("no CNI network configuration found in %s", m.cniPath.Conf)
	}
	m.defaultNetwork = netConfList[0]
	glog.V(1).Infof("Network configuration found: %s", m.defaultNetwork.Name)

	for _, p := range m.defaultNetwork.Plugins {
		if p.Network.Type == "loopback" {
			return nil
		}
	}

	glog.V(1).Infof("%s does not set up loopback interface, adding additional config", m.defaultNetwork.Name)
	m.loNetwork, _ = libcni.ConfListFromBytes([]byte(`
{
	"cniVersion": "0.3.1",
	"name": "sycri-loopback",
	"plugins": [{
        "type": "loopback"
	}]
}`))

	return nil
}

// SetUpPod bring up pod's network interface.
func (m *Manager) SetUpPod(podConfig *PodConfig) (*PodNetwork, error) {
	err := m.checkInit()
	if err != nil {
		return nil, err
	}
	if podConfig == nil {
		return nil, fmt.Errorf("nil POD configuration")
	}
	if podConfig.ID == "" {
		return nil, fmt.Errorf("empty ID")
	}
	if podConfig.NsPath == "" {
		return nil, fmt.Errorf("empty network namespace path")
	}
	if podConfig.Name == "" {
		return nil, fmt.Errorf("empty POD name")
	}
	if podConfig.Namespace == "" {
		return nil, fmt.Errorf("empty POD namespace name")
	}

	var cfg []*libcni.NetworkConfigList
	// add loopback interface if default network doesn't have one
	if m.loNetwork != nil {
		cfg = append(cfg, m.loNetwork)
	}
	cfg = append(cfg, m.defaultNetwork)
	setup, err := snetwork.NewSetupFromConfig(cfg, podConfig.ID, podConfig.NsPath, m.cniPath)
	if err != nil {
		return nil, err
	}

	args := fmt.Sprintf("%s:", m.defaultNetwork.Name)
	for i, kv := range [][2]string{
		{"IgnoreUnknown", "1"},
		{"K8S_POD_NAMESPACE", podConfig.Namespace},
		{"K8S_POD_NAME", podConfig.Name},
		{"K8S_POD_INFRA_CONTAINER_ID", podConfig.ID},
	} {
		if i > 0 {
			args += ";"
		}
		args += fmt.Sprintf("%s=%s", kv[0], kv[1])
	}
	if m.podCIDR != "" {
		args += fmt.Sprintf(";ipRange=%s", m.podCIDR)
	}
	if podConfig.PortMappings != nil {
		for _, pm := range podConfig.PortMappings {
			hostPort := pm.HostPort
			if hostPort == 0 {
				hostPort = pm.ContainerPort
			}
			err := setup.SetCapability(m.defaultNetwork.Name, "portMappings", snetwork.PortMapEntry{
				HostPort:      int(hostPort),
				ContainerPort: int(pm.ContainerPort),
				Protocol:      strings.ToLower(pm.Protocol.String()),
			})
			if err != nil {
				glog.Warningf("Skipping port mapping due to error: %v", err)
			}
		}
	}
	glog.V(3).Infof("Network for pod %s args: %s", podConfig.ID, args)
	if err := setup.SetArgs([]string{args}); err != nil {
		return nil, err
	}
	if err := setup.AddNetworks(); err != nil {
		return nil, err
	}
	return &PodNetwork{
		setup:          setup,
		defaultNetwork: m.defaultNetwork.Name,
	}, nil
}

// TearDownPod tears down pod's network interface.
func (m *Manager) TearDownPod(podNetwork *PodNetwork) error {
	if err := m.checkInit(); err != nil {
		return err
	}
	if podNetwork.setup == nil {
		return fmt.Errorf("nil network setup")
	}
	return podNetwork.setup.DelNetworks()
}

// Status returns an error if the network manager is not initialized.
func (m *Manager) Status() error {
	return m.checkInit()
}

// SetPodCIDR updates pod's CIDR.
func (m *Manager) SetPodCIDR(cidr string) {
	m.Lock()
	if m.podCIDR == "" {
		m.podCIDR = cidr
	}
	m.Unlock()
	m.checkInit()
}

// GetIP returns pod's IP address. It first tries to fetch IPv4
// and in case of errors will try to fetch IPv6.
func (n *PodNetwork) GetIP() (net.IP, error) {
	netIP, err := n.setup.GetNetworkIP(n.defaultNetwork, "4")
	if err == nil {
		return netIP, nil
	}

	netIP, err = n.setup.GetNetworkIP(n.defaultNetwork, "6")
	if err == nil {
		return netIP, nil
	}
	return nil, fmt.Errorf("could not get pod's IP: %v", err)
}
