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
	"context"
	"fmt"
	"log"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
	"github.com/sylabs/cri/pkg/singularity/runtime"
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
	log.Printf("launching observe server...")
	syncCtx, cancel := context.WithCancel(context.Background())
	p.syncCancel = cancel
	p.syncChan, err = runtime.ObserveState(syncCtx, p.socketPath())
	if err != nil {
		return fmt.Errorf("could not listen for state changes: %v", err)
	}

	go p.cli.Run(p.id, p.bundlePath(), "--empty-process", "--sync-socket", p.socketPath())

	if err := p.expectState(runtime.StateCreating); err != nil {
		return err
	}
	if err := p.expectState(runtime.StateCreated); err != nil {
		return err
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

func (p *Pod) expectState(expect runtime.State) error {
	log.Printf("waiting for state %d...", expect)
	p.runtimeState = <-p.syncChan
	if p.runtimeState != expect {
		return fmt.Errorf("unexpected pod state: %v", p.runtimeState)
	}
	return nil
}

func (p *Pod) cleanupRuntime(force bool) error {
	if p.runtimeState == runtime.StateExited {
		return nil
	}

	err := p.cli.Kill(p.id, force)
	if err != nil {
		return fmt.Errorf("could not treminate pod: %v", err)
	}
	err = p.expectState(runtime.StateExited)
	if err != nil {
		return err
	}
	p.syncCancel()
	if err := p.cli.Delete(p.id); err != nil {
		return fmt.Errorf("could not remove pod: %v", err)
	}
	return nil
}
