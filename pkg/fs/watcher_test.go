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

package fs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWatcher(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "fs-test")
	require.NoError(t, os.Mkdir(testDir, 0755))
	defer func() {
		require.NoError(t, os.RemoveAll(testDir), "could not remove test directory")
	}()

	file1 := filepath.Join(testDir, "test-watcher-1")
	file2 := filepath.Join(testDir, "test-watcher-2")
	file3 := filepath.Join(testDir, "test-watcher-3")

	f1, err := os.Create(file1)
	require.NoError(t, err, "could not create test file")
	require.NoError(t, f1.Close())

	f2, err := os.Create(file2)
	require.NoError(t, err, "could not create test file")
	require.NoError(t, f2.Close())

	ctx, cancel := context.WithCancel(context.Background())
	watcher, err := NewWatcher(testDir)
	require.NoError(t, err, "could not create watcher", err)
	upd := watcher.Watch(ctx)
	defer func() {
		cancel()
		require.Equalf(t, WatchEvent{}, <-upd, "unexpected update after close")
		require.NoError(t, watcher.Close(), "could not close watcher")
	}()

	require.NoError(t, os.Remove(file1), "could not remove test file")
	require.Equal(t, WatchEvent{
		Path: file1,
		Op:   OpRemove,
	}, <-upd, "unexpected update")

	require.NoError(t, os.Remove(file2), "could not remove test file")
	require.Equal(t, WatchEvent{
		Path: file2,
		Op:   OpRemove,
	}, <-upd, "unexpected update")

	f2, err = os.Create(file2)
	require.NoError(t, err, "could not create test file")
	require.NoError(t, f2.Close())
	require.Equal(t, WatchEvent{
		Path: file2,
		Op:   OpCreate,
	}, <-upd, "unexpected update")

	f3, err := os.Create(file3)
	require.NoError(t, err, "could not create test file")
	require.NoError(t, f3.Close())
	require.Equal(t, WatchEvent{
		Path: file3,
		Op:   OpCreate,
	}, <-upd, "unexpected update")

	file2New := file2 + "_new"
	require.NoError(t, os.Rename(file2, file2New), "could not rename test file")
	require.Equal(t, WatchEvent{
		Path: file2New,
		Op:   OpCreate,
	}, <-upd, "unexpected update")
}
