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

	"github.com/golang/glog"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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

func (p *Pod) baseDir() string {
	return filepath.Join(podInfoPath, p.id)
}

// hostnameFilePath returns path to pod's hostname file.
func (p *Pod) hostnameFilePath() string {
	return filepath.Join(p.baseDir(), podHostnamePath)
}

// resolvConfFilePath returns path to pod's resolv.conf file.
func (p *Pod) resolvConfFilePath() string {
	return filepath.Join(p.baseDir(), podResolvConfPath)
}

// bundlePath returns path to pod's filesystem bundle directory.
func (p *Pod) bundlePath() string {
	return filepath.Join(p.baseDir(), podBundlePath)
}

// rootfsPath returns path to pod's rootfs directory.
func (p *Pod) rootfsPath() string {
	return filepath.Join(p.baseDir(), podBundlePath, podRootfsPath)
}

// ociConfigPath returns path to pod's config.json file.
func (p *Pod) ociConfigPath() string {
	return filepath.Join(p.baseDir(), podBundlePath, podOCIConfigPath)
}

// socketPath returns path to pod's sync socket.
func (p *Pod) socketPath() string {
	return filepath.Join(p.baseDir(), podSocketPath)
}

// bindNamespacePath returns path to pod's namespace file of the passed type.
func (p *Pod) bindNamespacePath(nsType specs.LinuxNamespaceType) string {
	return filepath.Join(p.baseDir(), podNsStorePath, string(nsType))
}

func (p *Pod) prepareFiles() error {
	nsStorePath := filepath.Join(p.baseDir(), podNsStorePath)
	glog.V(8).Infof("Creating %s", nsStorePath)
	err := os.MkdirAll(nsStorePath, 0755)
	if err != nil {
		return fmt.Errorf("could not create directory for pod: %v", err)
	}
	if err := p.addLogDirectory(); err != nil {
		return fmt.Errorf("could not create log directory: %v", err)
	}
	if err := p.addResolvConf(); err != nil {
		return fmt.Errorf("could not create resolv.conf: %v", err)
	}
	if err := p.addHostname(); err != nil {
		return fmt.Errorf("could not create hostname file: %v", err)
	}
	return nil
}

func (p *Pod) addResolvConf() error {
	config := p.GetDnsConfig()
	if config == nil {
		return nil
	}

	glog.V(8).Infof("Creating resolv.conf file %s", p.resolvConfFilePath())
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
	glog.V(8).Infof("Creating hostname file %s", p.hostnameFilePath())
	host, err := os.OpenFile(p.hostnameFilePath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", podHostnamePath, err)
	}
	fmt.Fprintln(host, p.GetHostname())
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
	glog.V(8).Infof("Creating log directory %s", logDir)
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", logDir, err)
	}
	return nil
}

func (p *Pod) addOCIBundle() error {
	glog.V(8).Infof("Creating %s", p.rootfsPath())
	err := os.MkdirAll(p.rootfsPath(), 0755)
	if err != nil {
		return fmt.Errorf("could not create rootfs directory for pod: %v", err)
	}
	spec, err := translatePod(p)
	if err != nil {
		return fmt.Errorf("could not generate OCI spec for pod: %v", err)
	}
	glog.V(8).Infof("Creating oci config %s", p.ociConfigPath())
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
		glog.V(8).Infof("Removing binded namespace %s", ns.Path)
		err := namespace.Remove(ns)
		if err != nil && !silent {
			return fmt.Errorf("could not remove namespace: %v", err)
		}
	}
	glog.V(8).Infof("Removing pod base directory %s", p.baseDir())
	err := os.RemoveAll(p.baseDir())
	if err != nil && !silent {
		return fmt.Errorf("could not cleanup pod: %v", err)
	}
	glog.V(8).Infof("Removing pod log directory %s", p.GetLogDirectory())
	err = os.RemoveAll(p.GetLogDirectory())
	if err != nil && !silent {
		return fmt.Errorf("could not remove log directory: %v", err)
	}
	return nil
}
