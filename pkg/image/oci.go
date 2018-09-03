package image

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sylabs/cri/pkg/singularity"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type ociImageInfo struct {
	ref       string
	domain    string
	repo      string
	container string
	tags      []string
}

func parseOCIRef(ref string) ociImageInfo {
	ref = strings.TrimPrefix(ref, "docker://")
	refParts := strings.Split(ref, "/")

	info := ociImageInfo{
		ref:       "docker://" + ref,
		container: refParts[len(refParts)-1],
		tags:      []string{"latest"},
	}

	switch len(refParts) {
	case 3:
		info.domain = refParts[0]
		info.repo = refParts[1]
	case 2:
		info.repo = refParts[0]
	}

	imageParts := strings.Split(info.container, `:`)
	if len(imageParts) != 1 {
		info.container = imageParts[0]
		info.tags = strings.Split(imageParts[1], ",")
	}

	return info
}

func (i ociImageInfo) Remote() string {
	return i.ref
}

func (i ociImageInfo) Id() string {
	var parts []string
	if i.domain != "" {
		parts = append(parts, i.domain)
	}
	if i.repo != "" {
		parts = append(parts, i.repo)
	}
	parts = append(parts, i.container)
	return strings.Join(parts, "_") + ".sif"
}

func (i ociImageInfo) Tags() []string {
	return i.tags
}

func (i ociImageInfo) Digests() []string {
	return nil
}

func (i ociImageInfo) Pull(auth *v1alpha2.AuthConfig, dir string) error {
	path := filepath.Join(dir, i.Id())
	buildCmd := exec.Command(singularity.RuntimeName, "build", path, i.Remote())
	return buildCmd.Run()
}
