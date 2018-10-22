package runtime

import (
	"fmt"

	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func containerID(podID string, meta *k8s.ContainerMetadata) string {
	return fmt.Sprintf("%s_%s_%d", podID, meta.GetName(), meta.GetAttempt())
}
