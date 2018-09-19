package runtime

import "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

type pod struct {
	*v1alpha2.PodSandbox
	ns *v1alpha2.NamespaceOption
}

type podInfo struct {
	Pid  int `json:"pid"`
	PPid int `json:"ppid"`
}

func matchFilter(pod pod, filter *v1alpha2.PodSandboxFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != pod.Id {
		return false
	}

	if filter.State != nil && filter.State.State != pod.State {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := pod.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}
