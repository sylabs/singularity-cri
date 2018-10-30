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

package sandbox

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
	"github.com/sylabs/cri/pkg/rand"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
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
	pid        int
}

// New constructs Pod instance. Pod is thread safe to use.
func New(config *k8s.PodSandboxConfig) *Pod {
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
			if err := p.terminate(true); err != nil {
				log.Printf("could not kill pod failed run: %v", err)
			}
			if err := p.cleanup(true); err != nil {
				log.Printf("could not cleanup pod after failed run: %v", err)
			}
		}
	}()

	p.runOnce.Do(func() {
		if err = p.prepareFiles(); err != nil {
			err = fmt.Errorf("could not create pod directories: %v", err)
			return
		}
		if err = p.unshareNamespaces(); err != nil {
			err = fmt.Errorf("could not unshare namespaces: %v", err)
			return
		}
		if err = p.spawnOCIPod(); err != nil {
			err = fmt.Errorf("could not spawn pod: %v", err)
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
		err = p.terminate(false)
		if err != nil {
			err = fmt.Errorf("could not stop pod process: %v", err)
			return
		}
		p.state = k8s.PodSandboxState_SANDBOX_NOTREADY
	})
	return err
}

// Remove removes pod and all its containers, making sure nothing
// of it left on the host filesystem. When no Stop() is called before
// Remove forcibly kills all containers and pod itself.
func (p *Pod) Remove() error {
	var err error
	p.removeOnce.Do(func() {
		// todo remove containers
		err = p.terminate(true)
		if err != nil {
			err = fmt.Errorf("could not kill pod process: %v", err)
			return
		}
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

func (p *Pod) spawnOCIPod() error {
	spec, err := generateOCI(p)
	if err != nil {
		return fmt.Errorf("could not generate OCI spec for pod")
	}
	config, err := os.OpenFile(p.ociConfigPath(), os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("could not create OCI config file: %v", err)
	}
	defer config.Close()
	err = json.NewEncoder(config).Encode(spec)
	if err != nil {
		return fmt.Errorf("could not encode OCI config into json: %v", err)
	}

	//var errMsg bytes.Buffer
	//runCmd := exec.Command(singularity.RuntimeName, "oci", "create", p.bundlePath())
	//runCmd.Stderr = &errMsg
	//runCmd.Stdout = ioutil.Discard
	//err = runCmd.Run()
	//if err != nil {
	//	err = fmt.Errorf("could not spawn pod: %s", &errMsg)
	//}
	//// todo wait postStart hook here and get pid
	//
	//for i, ns := range p.namespaces {
	//	if ns.Type != specs.PIDNamespace {
	//		continue
	//	}
	//	p.namespaces[i].Path = p.bindNamespacePath(ns.Type)
	//	err := namespace.Bind(p.pid, p.namespaces[i])
	//	if err != nil {
	//		return fmt.Errorf("could not bind PID namespace: %v", err)
	//	}
	//}

	return nil
}

// terminate stops pod process if such exists with either SIGTERM
// or with SIGKIILL when force is true. It checks for process termination
// and in case signal was ignored it returns an error.
func (p *Pod) terminate(force bool) error {
	if p.pid == 0 {
		return nil
	}

	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	err := syscall.Kill(p.pid, sig)
	if err != nil {
		return fmt.Errorf("could not signal: %v", err)
	}

	var attempt int
	err = syscall.Kill(p.pid, 0)
	for err == nil && attempt < 10 {
		time.Sleep(time.Millisecond)
		err = syscall.Kill(p.pid, 0)
		attempt++
	}
	if err != nil && err != syscall.ESRCH {
		return fmt.Errorf("signaling failed: %v", err)
	}
	if attempt == 10 {
		return fmt.Errorf("signal ignored: %v", err)
	}
	p.pid = 0
	return nil
}

func (p *Pod) unshareNamespaces() error {
	if p.GetHostname() != "" {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.UTSNamespace,
			Path: p.bindNamespacePath(specs.UTSNamespace),
		})
	}
	security := p.GetLinux().GetSecurityContext()
	if security.GetNamespaceOptions().GetNetwork() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: p.bindNamespacePath(specs.NetworkNamespace),
		})
	}
	if security.GetNamespaceOptions().GetIpc() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.IPCNamespace,
			Path: p.bindNamespacePath(specs.IPCNamespace),
		})
	}
	if err := namespace.UnshareAll(p.namespaces); err != nil {
		return fmt.Errorf("unsahre all failed: %v", err)
	}
	// PID namespace is a special case, to create it pod process should be run
	if security.GetNamespaceOptions().GetPid() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.PIDNamespace,
		})
	}
	return nil
}
