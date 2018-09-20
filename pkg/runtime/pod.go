package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/sylabs/singularity/src/pkg/instance"
	"github.com/sylabs/singularity/src/pkg/sylog"
	syexec "github.com/sylabs/singularity/src/pkg/util/exec"
	"github.com/sylabs/singularity/src/runtime/engines/config"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type pod struct {
	ID        string
	Config    *v1alpha2.PodSandboxConfig
	State     v1alpha2.PodSandboxState
	CreatedAt int64
}

// RunPodSandbox creates and starts a pod-level sandbox. Runtimes must ensure
// the sandbox is in the ready state on success.
func (s *SingularityRuntime) RunPodSandbox(_ context.Context, req *v1alpha2.RunPodSandboxRequest) (*v1alpha2.RunPodSandboxResponse, error) {
	meta := req.Config.Metadata // assume metadata is always non-nil
	podID := fmt.Sprintf("%s_%s_%s_%d", meta.Name, meta.Namespace, meta.Uid, meta.Attempt)

	engineConf := config.Common{
		EngineName:   "podsandbox",
		EngineConfig: req.Config,
	}

	configData, err := json.Marshal(engineConf)
	if err != nil {
		return nil, fmt.Errorf("could not marshal engine config: %s", err)
	}

	envs := []string{sylog.GetEnvVar(), "SRUNTIME=podsandbox"}
	pipefd, err := syexec.SetPipe(configData)
	if err != nil {
		return nil, fmt.Errorf("could not configure pipe: %v", err)
	}
	envs = append(envs, pipefd)

	cmd := exec.Command("starter", podID)
	cmd.Env = envs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("will start pod now")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not start pod: %s", err)
	}
	log.Println("pod started")

	pod := pod{
		ID:        podID,
		Config:    req.Config,
		State:     v1alpha2.PodSandboxState_SANDBOX_READY,
		CreatedAt: time.Now().Unix(),
	}

	s.pods[podID] = pod
	return &v1alpha2.RunPodSandboxResponse{
		PodSandboxId: podID,
	}, nil
}

// StopPodSandbox stops any running process that is part of the sandbox and
// reclaims network resources (e.g., IP addresses) allocated to the sandbox.
// If there are any running containers in the sandbox, they must be forcibly
// terminated.
// This call is idempotent, and must not return an error if all relevant
// resources have already been reclaimed. kubelet will call StopPodSandbox
// at least once before calling RemovePodSandbox. It will also attempt to
// reclaim resources eagerly, as soon as a sandbox is not needed. Hence,
// multiple StopPodSandbox calls are expected.
func (s *SingularityRuntime) StopPodSandbox(_ context.Context, req *v1alpha2.StopPodSandboxRequest) (*v1alpha2.StopPodSandboxResponse, error) {
	// todo stop process?? free resources

	pod, ok := s.pods[req.PodSandboxId]
	if ok {
		pod.State = v1alpha2.PodSandboxState_SANDBOX_NOTREADY
		s.pods[pod.ID] = pod
	}
	return &v1alpha2.StopPodSandboxResponse{}, nil
}

// RemovePodSandbox removes the sandbox. If there are any running containers
// in the sandbox, they must be forcibly terminated and removed.
// This call is idempotent, and must not return an error if the sandbox has
// already been removed.
func (s *SingularityRuntime) RemovePodSandbox(_ context.Context, req *v1alpha2.RemovePodSandboxRequest) (*v1alpha2.RemovePodSandboxResponse, error) {
	pod, ok := s.pods[req.PodSandboxId]
	if !ok {
		return &v1alpha2.RemovePodSandboxResponse{}, nil
	}

	podI, err := instance.Get(pod.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read pod instance file: %v", err)
	}
	err = syscall.Kill(podI.Pid, syscall.SIGKILL)
	if err != nil {
		return nil, fmt.Errorf("could not stop pod: %v", err)
	}

	for err != syscall.ESRCH {
		// todo use epoll?
		err = syscall.Kill(podI.PPid, syscall.SIGKILL)
	}
	log.Println("monitor exited")

	_, err = os.Stat(podI.Path)
	if !os.IsNotExist(err) {
		err := os.Remove(podI.Path)
		if err != nil {
			return nil, fmt.Errorf("could not remove pod instance file: %v", err)
		}
	}
	delete(s.pods, pod.ID)
	return &v1alpha2.RemovePodSandboxResponse{}, nil
}

// PodSandboxStatus returns the status of the PodSandbox. If the PodSandbox is not
// present, returns an error.
func (s *SingularityRuntime) PodSandboxStatus(_ context.Context, req *v1alpha2.PodSandboxStatusRequest) (*v1alpha2.PodSandboxStatusResponse, error) {
	pod, ok := s.pods[req.PodSandboxId]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	var ns *v1alpha2.NamespaceOption
	if pod.Config.Linux != nil && pod.Config.Linux.SecurityContext != nil {
		ns = pod.Config.Linux.SecurityContext.NamespaceOptions
	}

	return &v1alpha2.PodSandboxStatusResponse{
		Status: &v1alpha2.PodSandboxStatus{
			Id:        pod.ID,
			Metadata:  pod.Config.Metadata,
			State:     pod.State,
			CreatedAt: pod.CreatedAt,
			Network:   nil, // todo
			Linux: &v1alpha2.LinuxPodSandboxStatus{
				Namespaces: &v1alpha2.Namespace{
					Options: ns,
				},
			},
			Labels:      pod.Config.Labels,
			Annotations: pod.Config.Annotations,
		},
	}, nil
}

// ListPodSandbox returns a list of PodSandboxes.
func (s *SingularityRuntime) ListPodSandbox(_ context.Context, req *v1alpha2.ListPodSandboxRequest) (*v1alpha2.ListPodSandboxResponse, error) {
	resp := &v1alpha2.ListPodSandboxResponse{}
	for _, pod := range s.pods {
		if matchFilter(pod, req.Filter) {
			resp.Items = append(resp.Items, &v1alpha2.PodSandbox{
				Id:          pod.ID,
				Metadata:    pod.Config.Metadata,
				State:       pod.State,
				CreatedAt:   pod.CreatedAt,
				Labels:      pod.Config.Labels,
				Annotations: pod.Config.Annotations,
			})
		}
	}
	return resp, nil
}

func matchFilter(pod pod, filter *v1alpha2.PodSandboxFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != pod.ID {
		return false
	}

	if filter.State != nil && filter.State.State != pod.State {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := pod.Config.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}
