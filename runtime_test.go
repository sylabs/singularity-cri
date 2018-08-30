// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sycri

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func TestSingularityRuntimeService_Version(t *testing.T) {
	s, err := NewSingularityRuntimeService()
	require.NoError(t, err, "could not create new runtime service")

	expectedVersion, err := exec.Command(s.singularity, "version").Output()
	require.NoError(t, err, "could not run version command against singularity")

	actualVersion, err := s.Version(context.Background(), &v1alpha2.VersionRequest{})
	require.NoError(t, err, "could not query runtime version")
	require.Equal(t, &v1alpha2.VersionResponse{
		Version:           kubeAPIVersion,
		RuntimeName:       singularityRuntimeName,
		RuntimeVersion:    string(expectedVersion),
		RuntimeApiVersion: string(expectedVersion),
	}, actualVersion, "runtime version mismatch")

}
