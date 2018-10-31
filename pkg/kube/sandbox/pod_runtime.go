package sandbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/kube"
	"github.com/sylabs/cri/pkg/singularity"
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

	//syncCtx, cancel := context.WithCancel(context.Background())
	//p.syncCancel = cancel
	//p.syncChan = kube.SyncWithRuntime(syncCtx, p.socketPath())
	//
	//var errMsg bytes.Buffer
	//runCmd := exec.Command(singularity.RuntimeName, "oci", "create", p.ID(), p.bundlePath())
	//runCmd.Stderr = &errMsg
	//runCmd.Stdout = ioutil.Discard
	//err = runCmd.Start()
	//if err != nil {
	//	return fmt.Errorf("could not run pod: %s", &errMsg)
	//}
	//defer runCmd.Wait()
	//
	//state := <-p.syncChan
	//if state != kube.StateCreating {
	//	return fmt.Errorf("unexpected pod state: %v", state)
	//}
	//state = <-p.syncChan
	//if state != kube.StateCreated {
	//	return fmt.Errorf("unexpected pod state: %v", state)
	//}
	//state = <-p.syncChan
	//if state != kube.StateRunning {
	//	return fmt.Errorf("unexpected pod state: %v", state)
	//}
	//
	//if err := runCmd.Wait(); err != nil {
	//	return fmt.Errorf("could not wait pod creation: %s", &errMsg)
	//}
	//
	//podState, err := p.queryState()
	//if err != nil {
	//	return fmt.Errorf("could not get pod pid: %v", err)
	//}
	//p.pid = podState.Pid
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

// nolint:unused
func (p *Pod) queryState() (*specs.State, error) {
	var state bytes.Buffer
	stateCmd := exec.Command(singularity.RuntimeName, "oci", "state", "--json", p.ID())
	stateCmd.Stderr = ioutil.Discard
	stateCmd.Stdout = &state

	if err := stateCmd.Run(); err != nil {
		return nil, fmt.Errorf("could not query pod state: %v", err)
	}

	var podState *specs.State
	err := json.Unmarshal(state.Bytes(), &podState)
	if err != nil {
		return nil, fmt.Errorf("could not decode pod state: %v", err)
	}
	return podState, nil
}

func (p *Pod) cleanupRuntime(force bool) error {
	if p.pid == 0 {
		return nil
	}
	p.syncCancel()
	err := kube.Terminate(p.pid, force)
	if err != nil {
		return fmt.Errorf("could not treminate pod: %v", err)
	}
	p.pid = 0
	return nil
}
