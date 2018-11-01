package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
	"github.com/sylabs/cri/pkg/singularity/runtime"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func (p *Pod) spawnOCIPod() error {
	// PID namespace is a special case, to create it pod process should be run
	if p.GetLinux().GetSecurityContext().GetNamespaceOptions().GetPid() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.PIDNamespace,
		})
	}

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

	log.Printf("launching observe server...")
	syncCtx, cancel := context.WithCancel(context.Background())
	p.syncCancel = cancel
	p.syncChan, err = runtime.ObserveState(syncCtx, p.socketPath())
	if err != nil {
		return fmt.Errorf("could not listen for state changes: %v", err)
	}

	go p.cli.Run(p.id, p.bundlePath())

	log.Printf("waiting for creating...")
	state := <-p.syncChan
	if state != runtime.StateCreating {
		return fmt.Errorf("unexpected pod state: %v", state)
	}
	log.Printf("waiting for created...")
	state = <-p.syncChan
	if state != runtime.StateCreated {
		return fmt.Errorf("unexpected pod state: %v", state)
	}
	log.Printf("waiting for running...")
	state = <-p.syncChan
	if state != runtime.StateRunning {
		return fmt.Errorf("unexpected pod state: %v", state)
	}

	log.Printf("query state...")
	podState, err := p.cli.State(p.id)
	if err != nil {
		return fmt.Errorf("could not get pod pid: %v", err)
	}

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
	return nil
}

func (p *Pod) cleanupRuntime(force bool) error {
	p.syncCancel()
	err := p.cli.Kill(p.id, force)
	if err != nil {
		return fmt.Errorf("could not treminate pod: %v", err)
	}
	return nil
}
