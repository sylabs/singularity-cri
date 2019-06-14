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
	"context"
	"fmt"

	"github.com/golang/glog"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/singularity-cri/pkg/namespace"
	"github.com/sylabs/singularity-cri/pkg/singularity/runtime"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func (p *Pod) spawnOCIPod() error {
	// PID namespace is a special case, to create it pod process should be run
	podPID := p.GetLinux().GetSecurityContext().GetNamespaceOptions().GetPid() == k8s.NamespaceMode_POD
	if podPID {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.PIDNamespace,
		})
	}

	err := p.addOCIBundle()
	if err != nil {
		return fmt.Errorf("could not create oci bundle: %v", err)
	}

	syncCtx, cancel := context.WithCancel(context.Background())
	p.syncCancel = cancel
	p.syncChan, err = runtime.ObserveState(syncCtx, p.socketPath())
	if err != nil {
		return fmt.Errorf("could not listen for state changes: %v", err)
	}

	glog.V(10).Infof("Creating pod %s", p.id)
	_, err = p.cli.Create(p.id, p.bundlePath(), false, "--empty-process", "--sync-socket", p.socketPath())
	if err != nil {
		return fmt.Errorf("could not create pod: %v", err)
	}

	if err := p.expectState(runtime.StateCreating); err != nil {
		return err
	}
	if err := p.expectState(runtime.StateCreated); err != nil {
		return err
	}

	glog.V(10).Infof("Starting pod %s", p.id)
	if err := p.cli.Start(p.id); err != nil {
		return fmt.Errorf("could not start pod: %v", err)
	}

	if err := p.expectState(runtime.StateRunning); err != nil {
		return err
	}

	podState, err := p.cli.State(p.id)
	if err != nil {
		return fmt.Errorf("could not get pod pid: %v", err)
	}

	if podPID {
		for i, ns := range p.namespaces {
			if ns.Type != specs.PIDNamespace {
				continue
			}
			p.namespaces[i].Path = p.bindNamespacePath(ns.Type)
			err := namespace.Bind(podState.Pid, p.namespaces[i])
			if err != nil {
				return fmt.Errorf("could not bind PID namespace: %v", err)
			}
		}
	}
	return nil
}

// UpdateState updates container state according to information
// received from the runtime.
func (p *Pod) UpdateState() error {
	var err error
	p.ociState, err = p.cli.State(p.id)
	if err != nil {
		return fmt.Errorf("could not get pod state: %v", err)
	}
	p.runtimeState = runtime.StatusToState(p.ociState.Status)
	return nil
}

// Pid returns pid of the pod process in the host's PID namespace.
func (p *Pod) Pid() int {
	return p.ociState.Pid
}

func (p *Pod) expectState(expect runtime.State) error {
	p.runtimeState = <-p.syncChan
	if p.runtimeState != expect {
		return fmt.Errorf("unexpected pod state: %v", p.runtimeState)
	}
	return nil
}

func (p *Pod) terminate(force bool) error {
	// Call cancel to free any resources taken by context.
	// We should call it when sync socket will no longer be used, and
	// since multiple calls are fine with cancel func, call it at
	// the end of terminate.
	if p.syncCancel != nil {
		defer p.syncCancel()
	}

	if p.runtimeState == runtime.StateExited {
		return nil
	}

	if force {
		glog.V(4).Infof("Forcibly stopping pod %s", p.id)
	} else {
		glog.V(4).Infof("Terminating pod %s", p.id)
	}
	err := p.cli.Kill(p.id, force)
	if err != nil {
		return fmt.Errorf("could not terminate pod: %v", err)
	}
	return p.expectState(runtime.StateExited)
}
