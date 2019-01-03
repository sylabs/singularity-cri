// Copyright (c) 2018 Sylabs, Inc. All rights reserved.
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

package fs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMountPoint(t *testing.T) {
	tt := []struct {
		path   string
		parent string
		err    error
	}{
		{
			path: "/proc_",
			err:  fmt.Errorf("could not resolve path /proc_: lstat /proc_: no such file or directory"),
		},
		{
			path:   "/proc",
			parent: "/proc",
		},
		{
			path:   "/home",
			parent: "/",
		},
		{
			path:   "/dev/null",
			parent: "/dev",
		},
		{
			path:   "/var/run/mount",
			parent: "/run",
		},
		{
			path:   "/var/lib",
			parent: "/",
		},
		{
			path:   "/proc/self",
			parent: "/proc",
		},
		{
			path: "/proc/fake",
			err:  fmt.Errorf("could not resolve path /proc/fake: lstat /proc/fake: no such file or directory"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.path, func(t *testing.T) {
			parent, err := MountPoint(tc.path)
			require.Equal(t, tc.parent, parent)
			require.Equal(t, tc.err, err)
		})
	}
}
