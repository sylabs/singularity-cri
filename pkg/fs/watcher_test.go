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
	file1 := filepath.Join(os.TempDir(), "test-watcher-1")
	file2 := filepath.Join(os.TempDir(), "test-watcher-2")
	file3 := filepath.Join(os.TempDir(), "test-watcher-3")

	f1, err := os.Create(file1)
	require.NoError(t, err, "could not create test file")
	require.NoError(t, f1.Close())

	f2, err := os.Create(file2)
	require.NoError(t, err, "could not create test file")
	require.NoError(t, f2.Close())

	ctx, cancel := context.WithCancel(context.Background())
	watcher, err := NewWatcher(os.TempDir())
	require.NoError(t, err, "could not create watcher", err)
	defer func() {
		cancel()
		require.NoError(t, watcher.Close(), "could not close watcher")
	}()

	upd := watcher.Watch(ctx)

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
	select {
	case <-upd:
		t.Fatal("Unexpected event after rename")
	default:
	}
	require.NoError(t, os.Remove(file2New), "could not remove renamed file")
	require.NoError(t, os.Remove(file3), "could not remove test file")
}
