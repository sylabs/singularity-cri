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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
)

const (
	podInfoPath = "/var/run/singularity/pods/"

	podNsStorePath    = "namespaces/"
	podResolvConfPath = "resolv.conf"
	podHostnamePath   = "hostname"
	podSocketPath     = "sync.sock"

	podBundlePath    = "bundle/"
	podRootfsPath    = "rootfs/"
	podOCIConfigPath = "config.json"
)

// namespacePath returns path to pod's namespace file of the passed type.
// If requested namespace is not unshared specifically for pod an empty
// string is returned.
func (p *Pod) namespacePath(nsType specs.LinuxNamespaceType) string {
	for _, ns := range p.namespaces {
		if ns.Type == nsType {
			return p.bindNamespacePath(nsType)
		}
	}
	return ""
}

// hostnameFilePath returns path to pod's hostname file.
func (p *Pod) hostnameFilePath() string {
	return filepath.Join(podInfoPath, p.id, podHostnamePath)
}

// resolvConfFilePath returns path to pod's resolv.conf file.
func (p *Pod) resolvConfFilePath() string {
	return filepath.Join(podInfoPath, p.id, podResolvConfPath)
}

// bundlePath returns path to pod's filesystem bundle directory.
func (p *Pod) bundlePath() string {
	return filepath.Join(podInfoPath, p.id, podBundlePath)
}

// rootfsPath returns path to pod's rootfs directory.
func (p *Pod) rootfsPath() string {
	return filepath.Join(podInfoPath, p.id, podBundlePath, podRootfsPath)
}

// ociConfigPath returns path to pod's config.json file.
func (p *Pod) ociConfigPath() string {
	return filepath.Join(podInfoPath, p.id, podBundlePath, podOCIConfigPath)
}

// socketPath returns path to pod's sync socket.
func (p *Pod) socketPath() string {
	return filepath.Join(podInfoPath, p.id, podSocketPath)
}

// bindNamespacePath returns path to pod's namespace file of the passed type.
func (p *Pod) bindNamespacePath(nsType specs.LinuxNamespaceType) string {
	return filepath.Join(podInfoPath, p.id, podNsStorePath, string(nsType))
}

func (p *Pod) prepareFiles() error {
	err := os.MkdirAll(filepath.Join(podInfoPath, p.id, podNsStorePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create directory for pod: %v", err)
	}
	if err = p.addLogDirectory(); err != nil {
		return fmt.Errorf("could not create log directory: %v", err)
	}
	if err = p.addResolvConf(); err != nil {
		return fmt.Errorf("could not create resolv.conf: %v", err)
	}
	if err = p.addHostname(); err != nil {
		return fmt.Errorf("could not create hostname file: %v", err)
	}
	return nil
}

func (p *Pod) addResolvConf() error {
	config := p.GetDnsConfig()
	if config == nil {
		return nil
	}

	resolv, err := os.OpenFile(p.resolvConfFilePath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", podResolvConfPath, err)
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
		return fmt.Errorf("could not close %s: %v", podResolvConfPath, err)
	}
	return nil
}

func (p *Pod) addHostname() error {
	hostname := p.GetHostname()
	if hostname == "" {
		return nil
	}

	host, err := os.OpenFile(p.hostnameFilePath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", podHostnamePath, err)
	}
	fmt.Fprintln(host, hostname)
	if err = host.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", podHostnamePath, err)
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

func (p *Pod) addOCIBundle() error {
	err := os.MkdirAll(p.rootfsPath(), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create rootfs directory for pod: %v", err)
	}
	spec, err := translatePod(p)
	if err != nil {
		return fmt.Errorf("could not generate OCI spec for pod: %v", err)
	}
	config, err := os.OpenFile(p.ociConfigPath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create OCI config file: %v", err)
	}
	defer config.Close()
	err = json.NewEncoder(config).Encode(spec)
	if err != nil {
		return fmt.Errorf("could not encode OCI config into json: %v", err)
	}
	return nil
}

// cleanupFiles is responsible for cleaning any files that were created by pod.
// If silent is true then any errors occurred during cleanupFiles are ignored.
func (p *Pod) cleanupFiles(silent bool) error {
	for _, ns := range p.namespaces {
		err := namespace.Remove(ns)
		if err != nil && !silent {
			return fmt.Errorf("could not remove namespace: %v", err)
		}
	}
	err := os.RemoveAll(filepath.Join(podInfoPath, p.id))
	if err != nil && !silent {
		return fmt.Errorf("could not cleanup pod: %v", err)
	}
	err = os.RemoveAll(p.GetLogDirectory())
	if err != nil && !silent {
		return fmt.Errorf("could not remove log directory: %v", err)
	}
	return nil
}
