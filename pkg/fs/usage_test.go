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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsage(t *testing.T) {
	t.Run("non-existent path", func(t *testing.T) {
		info, err := Usage("/proc/fake")
		require.Nil(t, info)
		require.Equal(t, fmt.Errorf("could not get mount point: could not resolve path /proc/fake: lstat /proc/fake: no such file or directory"),
			err)
	})

	t.Run("all ok", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "usage-test")
		require.NoError(t, err, "could not create temp dir")
		defer os.RemoveAll(dir)

		d, err := os.Open(dir)
		require.NoError(t, err, "could not open temp directory")
		defer d.Close()
		dStat, err := d.Stat()
		require.NoError(t, err, "could not get temp directory stat")

		c1 := []byte("Happy New Year!")
		f1 := filepath.Join(dir, "file1")
		err = ioutil.WriteFile(f1, c1, 0666)
		require.NoError(t, err, "could not create temp file 1")

		c2 := []byte("Merry Christmas from Singularity team!")
		f2 := filepath.Join(dir, "file2")
		err = ioutil.WriteFile(f2, c2, 0666)
		require.NoError(t, err, "could not create temp file 2")

		innerDir := filepath.Join(dir, "inner")
		err = os.Mkdir(innerDir, 0755)
		require.NoError(t, err, "could not create inner dir")
		in, err := os.Open(dir)
		require.NoError(t, err, "could not open inner temp directory")
		defer in.Close()
		inStat, err := in.Stat()
		require.NoError(t, err, "could not get inner temp directory stat")

		c3 := []byte("k8s+singularity")
		f3 := filepath.Join(innerDir, "file3")
		err = ioutil.WriteFile(f3, c3, 0666)
		require.NoError(t, err, "could not create temp file 3")

		info, err := Usage(dir)
		require.NoError(t, err, "could not get directory usage")

		require.Equal(t, &UsageInfo{
			MountPoint: "/",
			Bytes:      int64(len(c1)+len(c2)+len(c3)) + dStat.Size() + inStat.Size(),
			Inodes:     5,
		}, info)
	})
}
