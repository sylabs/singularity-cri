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
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
	"github.com/sylabs/cri/pkg/rand"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	podInfoPathFormat = "/var/run/singularity/pods/"
	nsStorePathFormat = "namespaces/"
	resolvConfPath    = "resolv.conf"
	hostnamePath      = "hostname"

	podIDLen = 64
)

// Pod represents kubernetes pod. It encapsulates all pod-specific
// logic and should be used by runtime for correct interaction.
type Pod struct {
	id string
	*k8s.PodSandboxConfig

	runOnce    sync.Once
	stopOnce   sync.Once
	removeOnce sync.Once

	state      k8s.PodSandboxState
	createdAt  int64 // unix nano
	namespaces []specs.LinuxNamespace
}

// NewPod constructs Pod instance. Pod is thread safe to use.
func NewPod(config *k8s.PodSandboxConfig) *Pod {
	podID := rand.GenerateID(podIDLen)
	return &Pod{
		PodSandboxConfig: config,
		id:               podID,
		state:            k8s.PodSandboxState_SANDBOX_NOTREADY,
	}
}

// ID returns unique pod ID.
func (p *Pod) ID() string {
	return p.id
}

// Run prepares and runs pod based on initial config passed to NewPod.
func (p *Pod) Run() error {
	var err error
	defer func() {
		if err != nil {
			p.cleanup(true)
		}
	}()

	p.runOnce.Do(func() {
		if err = p.ensureDirectories(); err != nil {
			err = fmt.Errorf("could not create log directory: %v", err)
			return
		}
		if err = p.addLogDirectory(); err != nil {
			err = fmt.Errorf("could not create log directory: %v", err)
			return
		}
		if err = p.addResolvConf(); err != nil {
			err = fmt.Errorf("could not configure dns: %v", err)
			return
		}
		if err = p.addHostname(); err != nil {
			err = fmt.Errorf("could not configure hostname: %v", err)
			return
		}
		if err = p.unshareNamespaces(); err != nil {
			err = fmt.Errorf("could not unshare namespaces: %v", err)
			return
		}
		p.state = k8s.PodSandboxState_SANDBOX_READY
		p.createdAt = time.Now().UnixNano()
	})
	return err
}

// Stop stops pod and all its containers, reclaims any resources.
func (p *Pod) Stop() error {
	if p.state == k8s.PodSandboxState_SANDBOX_NOTREADY {
		return nil
	}

	var err error
	p.stopOnce.Do(func() {
		// todo stop containers
		// todo reclaim resources somewhere here
		p.state = k8s.PodSandboxState_SANDBOX_NOTREADY
	})
	return err
}

// Remove removes pod and all its containers, making sure nothing
// of it left on the host filesystem.
func (p *Pod) Remove() error {
	var err error
	p.removeOnce.Do(func() {
		// todo remove containers
		if err = p.cleanup(false); err != nil {
			err = fmt.Errorf("could not cleanup pod: %v", err)
		}
	})
	return err
}

// State returns current pod state.
func (p *Pod) State() k8s.PodSandboxState {
	return p.state
}

// CreatedAt returns pod creation time in Unix nano.
func (p *Pod) CreatedAt() int64 {
	return p.createdAt
}

// BindNamespacePath returns path to pod's namespace file of the passed type.
func (p *Pod) BindNamespacePath(nsType specs.LinuxNamespaceType) string {
	return filepath.Join(podInfoPathFormat, p.id, nsStorePathFormat, string(nsType))
}

// HostnameFilePath returns path to pod's hostname file.
func (p *Pod) HostnameFilePath() string {
	return filepath.Join(podInfoPathFormat, p.id, hostnamePath)
}

// ResolvConfFilePath returns path to pod's resolv.conf file.
func (p *Pod) ResolvConfFilePath() string {
	return filepath.Join(podInfoPathFormat, p.id, resolvConfPath)
}

// MatchesFilter tests Pod against passed filter and returns true if it matches.
func (p *Pod) MatchesFilter(filter *k8s.PodSandboxFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != p.id {
		return false
	}

	if filter.State != nil && filter.State.State != p.state {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := p.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}

// cleanup is responsible for cleaning any files that were created by pod.
// If noErr is true then any errors occurred during cleanup are ignored.
func (p *Pod) cleanup(noErr bool) error {
	for _, ns := range p.namespaces {
		err := namespace.Remove(ns)
		if err != nil && !noErr {
			return fmt.Errorf("could not remove namespace: %v", err)
		}
	}
	err := os.RemoveAll(filepath.Join(podInfoPathFormat, p.id))
	if err != nil && !noErr {
		return fmt.Errorf("could not cleanup pod: %v", err)
	}
	if p.GetLogDirectory() != "" {
		err := os.RemoveAll(p.GetLogDirectory())
		if err != nil && !noErr {
			return fmt.Errorf("could not remove log directory: %v", err)
		}
	}
	return nil
}

func (p *Pod) addResolvConf() error {
	config := p.GetDnsConfig()
	if config == nil {
		return nil
	}

	resolv, err := os.Create(p.ResolvConfFilePath())
	if err != nil {
		return fmt.Errorf("could not create %s: %v", resolvConfPath, err)
	}
	for _, s := range config.GetServers() {
		fmt.Fprintf(resolv, "nameserver %s\n", s)
	}
	for _, s := range config.GetSearches() {
		fmt.Fprintf(resolv, "search %s\n", s)
	}
	for _, o := range config.GetOptions() {
		fmt.Fprintf(resolv, "options %s\n", o)
	}
	if err = resolv.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", resolvConfPath, err)
	}
	return nil
}

func (p *Pod) addHostname() error {
	hostname := p.GetHostname()
	if hostname == "" {
		return nil
	}

	host, err := os.Create(p.HostnameFilePath())
	if err != nil {
		return fmt.Errorf("could not create %s: %v", hostnamePath, err)
	}
	fmt.Fprintln(host, hostname)
	if err = host.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", hostnamePath, err)
	}
	return nil
}

func (p *Pod) addLogDirectory() error {
	logDir := p.GetLogDirectory()
	if logDir == "" {
		return nil
	}

	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", logDir, err)
	}
	return nil
}

func (p *Pod) unshareNamespaces() error {
	if p.GetHostname() != "" {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.UTSNamespace,
			Path: p.BindNamespacePath(specs.UTSNamespace),
		})
	}
	security := p.GetLinux().GetSecurityContext()
	if security.GetNamespaceOptions().GetNetwork() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: p.BindNamespacePath(specs.NetworkNamespace),
		})
	}
	if security.GetNamespaceOptions().GetIpc() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.IPCNamespace,
			Path: p.BindNamespacePath(specs.IPCNamespace),
		})
	}
	if err := namespace.UnshareAll(p.namespaces); err != nil {
		return fmt.Errorf("unsahre all failed: %v", err)
	}
	return nil
}

func (p *Pod) ensureDirectories() error {
	err := os.MkdirAll(filepath.Join(podInfoPathFormat, p.id, nsStorePathFormat), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create directory for pod: %v", err)
	}
	return nil
}
