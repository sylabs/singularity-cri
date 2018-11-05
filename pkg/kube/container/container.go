package container

import (
	"encoding/json"
	"fmt"
	"log"

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
}

// New constructs Container instance. Container is thread safe to use.
func New(config *k8s.ContainerConfig) *Container {
	contID := rand.GenerateID(contIDLen)
	return &Container{
		id:              contID,
		ContainerConfig: config,
	}
}

// ID returns unique container ID.
func (c *Container) ID() string {
	return c.id
}

// Create creates container inside a pod from the image.
func (c *Container) Create(image *image.Info, pod *sandbox.Pod) error {
	ociSpec, err := generateOCI(c, pod)
	if err != nil {
		return fmt.Errorf("could not generate oci config: %v", err)
	}
	ociBytes, err := json.Marshal(ociSpec)
	if err != nil {
		return fmt.Errorf("could not marshal oci config: %v", err)
	}
	log.Printf("OCI config is: %s", ociBytes)
	// todo oci bundle here

	return nil
}
