package container

import (
	"fmt"
	"sync"
	"time"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/kube/sandbox"
	"github.com/sylabs/cri/pkg/rand"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	contIDLen = 64
)

// Container represents kubernetes container inside a pod. It encapsulates
// all container-specific logic and should be used by runtime for correct interaction.
type Container struct {
	id string
	*k8s.ContainerConfig
	pod *sandbox.Pod

	createdAt int64 // unix nano

	createOnce sync.Once

	state k8s.ContainerState
}

// New constructs Container instance. Container is thread safe to use.
func New(config *k8s.ContainerConfig, pod *sandbox.Pod) *Container {
	contID := rand.GenerateID(contIDLen)
	return &Container{
		id:              contID,
		ContainerConfig: config,
		pod:             pod,
	}
}

// ID returns unique container ID.
func (c *Container) ID() string {
	return c.id
}

// PodID returns ID of a pod contaienr is executed in.
func (c *Container) PodID() string {
	return c.pod.ID()
}

// State returns current pod state.
func (c *Container) State() k8s.ContainerState {
	return c.state
}

// CreatedAt returns pod creation time in Unix nano.
func (c *Container) CreatedAt() int64 {
	return c.createdAt
}

// Create creates container inside a pod from the image.
func (c *Container) Create(info *image.Info) error {
	var err error
	defer func() {
		if err != nil {
			// todo cleanup
		}
	}()

	c.createOnce.Do(func() {
		err = c.addOCIBundle(info)
		if err != nil {
			err = fmt.Errorf("could not prepare oci bundle: %v", err)
			return
		}
		c.createdAt = time.Now().UnixNano()
		c.pod.AddContainer(c)
	})
	return err
}

// MatchesFilter tests Container against passed filter and returns true if it matches.
func (c *Container) MatchesFilter(filter *k8s.ContainerFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != c.id {
		return false
	}

	if filter.PodSandboxId != "" && filter.PodSandboxId != c.pod.ID() {
		return false
	}

	if filter.State != nil && filter.State.State != c.state {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := c.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}
