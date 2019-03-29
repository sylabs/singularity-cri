// Copyright (c) 2018-2019 Sylabs, Inc. All rights reserved.
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

package namespace

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
)

func TestBindRemove(t *testing.T) {
	myPid := os.Getpid()

	t.Run("create target file error", func(t *testing.T) {
		ns := specs.LinuxNamespace{
			Type: specs.PIDNamespace,
			Path: "/not/found",
		}
		err := Bind(myPid, ns)
		require.Equal(t, fmt.Errorf("could not create /not/found: open /not/found: no such file or directory"), err)
	})

	t.Run("bind error", func(t *testing.T) {
		ns := specs.LinuxNamespace{
			Type: specs.MountNamespace,
			Path: filepath.Join(os.TempDir(), "test-ns"),
		}
		require.Equal(t, fmt.Errorf("could not mount /proc/%d/ns/mnt: invalid argument", myPid), Bind(os.Getpid(), ns))
	})

	t.Run("all ok", func(t *testing.T) {
		ns := specs.LinuxNamespace{
			Type: specs.IPCNamespace,
			Path: filepath.Join(os.TempDir(), "test-ns"),
		}
		require.NoError(t, Bind(os.Getpid(), ns))
		require.NoError(t, Remove(ns))
	})
}
