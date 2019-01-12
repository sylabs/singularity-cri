// Copyright (c) 2019 Sylabs, Inc. All rights reserved.
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
	"strings"
	"sync"

	"github.com/golang/glog"

	"github.com/containernetworking/cni/libcni"

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
	defaultNetwork *libcni.NetworkConfigList
	cniPath        *snetwork.CNIPath
	podCidr        string
}

// PodNetworkConfig contains/defines POD network configuration.
type PodNetworkConfig struct {
	ID           string
	Namespace    string
	Name         string
	NsPath       string
	PortMappings []*k8s.PortMapping
	Setup        *snetwork.Setup
}

// Init initialize CNI network manager.
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

// checkInit updates CNI network configuration and does some
// sanity checks.
func (m *Manager) checkInit() error {
	if err := m.setDefaultNetwork(); err != nil {
		return err
	}

	m.Lock()
	defer m.Unlock()

	ipRanges := false
	for _, plugin := range m.defaultNetwork.Plugins {
		if plugin.Network.Capabilities["ipRanges"] {
			if m.podCidr == "" {
				return fmt.Errorf("no PodCIDR set")
			}
			ipRanges = true
			break
		}
	}
	if !ipRanges && m.podCidr != "" {
		glog.Infof("Reset pod CIDR, network plugin don't support it")
		m.podCidr = ""
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
		return fmt.Errorf("no CNI network configuration found in %s", m.cniPath.Conf)
	}
	if len(netConfList) == 0 {
		return fmt.Errorf("no CNI network configuration found in %s", m.cniPath.Conf)
	}
	m.defaultNetwork = netConfList[0]
	glog.Infof("Network configuration found: %s", m.defaultNetwork.Name)
	return nil
}

// SetUpPod bring up POD network interface.
func (m *Manager) SetUpPod(podNetworkConfig *PodNetworkConfig) error {
	err := m.checkInit()
	if err != nil {
		return err
	}
	if podNetworkConfig == nil {
		return fmt.Errorf("nil POD configuration")
	}
	if podNetworkConfig.ID == "" {
		return fmt.Errorf("empty ID")
	}
	if podNetworkConfig.NsPath == "" {
		return fmt.Errorf("empty network namespace path")
	}
	if podNetworkConfig.Name == "" {
		return fmt.Errorf("empty POD name")
	}
	if podNetworkConfig.Name == "" {
		return fmt.Errorf("empty POD name")
	}
	if podNetworkConfig.Namespace == "" {
		return fmt.Errorf("empty POD namespace name")
	}

	cfg := []*libcni.NetworkConfigList{m.defaultNetwork}
	podNetworkConfig.Setup, err = snetwork.NewSetupFromConfig(cfg, podNetworkConfig.ID, podNetworkConfig.NsPath, m.cniPath)
	if err != nil {
		return err
	}

	args := fmt.Sprintf("%s:", cfg[0].Name)
	for i, kv := range [][2]string{
		{"IgnoreUnknown", "1"},
		{"K8S_POD_NAMESPACE", podNetworkConfig.Namespace},
		{"K8S_POD_NAME", podNetworkConfig.Name},
		{"K8S_POD_INFRA_CONTAINER_ID", podNetworkConfig.ID},
	} {
		if i > 0 {
			args += ";"
		}
		args += fmt.Sprintf("%s=%s", kv[0], kv[1])
	}
	if m.podCidr != "" {
		args += fmt.Sprintf(";ipRange=%s", m.podCidr)
	}
	if podNetworkConfig.PortMappings != nil {
		for _, pm := range podNetworkConfig.PortMappings {
			hostport := pm.HostPort
			if hostport == 0 {
				hostport = pm.ContainerPort
			}
			args += fmt.Sprintf(";portmap=%d:%d/%s", hostport, pm.ContainerPort, strings.ToLower(pm.Protocol.String()))
		}
	}
	glog.Infof("Network args: %s", args)
	if err := podNetworkConfig.Setup.SetArgs([]string{args}); err != nil {
		return err
	}
	if err := podNetworkConfig.Setup.AddNetworks(); err != nil {
		return err
	}
	return nil
}

// TearDownPod tears down POD network interface.
func (m *Manager) TearDownPod(podNetworkConfig *PodNetworkConfig) error {
	if err := m.checkInit(); err != nil {
		return err
	}
	if podNetworkConfig.Setup == nil {
		return fmt.Errorf("nil network setup")
	}
	return podNetworkConfig.Setup.DelNetworks()
}

// Status returns an error if the network manager is not initialized.
func (m *Manager) Status() error {
	return m.checkInit()
}

// SetPodCIDR updates POD CIDR.
func (m *Manager) SetPodCIDR(cidr string) {
	m.Lock()
	if m.podCidr == "" {
		m.podCidr = cidr
	}
	m.Unlock()
	m.checkInit()
}
