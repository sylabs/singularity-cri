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

package index

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sylabs/singularity-cri/pkg/kube"
)

func TestPodIndex(t *testing.T) {
	indx := NewPodIndex()

	busybox := kube.NewPod(nil)
	nginx := kube.NewPod(nil)
	alpine := kube.NewPod(nil)

	t.Run("empty index", func(t *testing.T) {
		found, err := indx.Find(busybox.ID())
		require.Equal(t, ErrNotFound, err, "empty index didn't return ErrNotFound")
		require.Nil(t, found, "empty index returned pod")

		found, err = indx.Find("illegal id")
		require.EqualError(t, err, "could not search index: illegal character: ' '",
			"illegal id didn't lead to error")
		require.Nil(t, found, "illegal id returned pod")

		err = indx.Remove(busybox.ID())
		require.NoError(t, err, "empty index returned error on remove")
		err = indx.Remove("illegal id")
		require.EqualError(t, err, "could not remove pod: illegal character: ' '",
			"illegal id didn't lead to error")
	})

	t.Run("add to empty index", func(t *testing.T) {
		err := indx.Add(busybox)
		require.NoError(t, err)
		err = indx.Add(nginx)
		require.NoError(t, err)
		err = indx.Add(alpine)
		require.NoError(t, err)
	})

	t.Run("search non-empty index", func(t *testing.T) {
		found, err := indx.Find(busybox.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found, busybox, "index returned wrong pod")

		found, err = indx.Find(nginx.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found, nginx, "index returned wrong pod")

		found, err = indx.Find(alpine.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found, alpine, "index returned wrong pod")

		found, err = indx.Find("nonExistentID")
		require.Equal(t, ErrNotFound, err, "index didn't return ErrNotFound")
		require.Nil(t, found, "index returned unexpected pod")
	})

	t.Run("remove from index", func(t *testing.T) {
		err := indx.Remove(nginx.ID())
		require.NoError(t, err, "could not remove pod from index")

		found, err := indx.Find(nginx.ID())
		require.Equal(t, ErrNotFound, err, "index didn't return ErrNotFound")
		require.Nil(t, found, "removed pod is still returned")

		err = indx.Remove(nginx.ID())
		require.NoError(t, err, "removing removed pod lead to error")
	})

	t.Run("iterate pods", func(t *testing.T) {
		var count int
		indx.Iterate(func(pod *kube.Pod) {
			count++
			if pod.ID() == busybox.ID() ||
				pod.ID() == alpine.ID() {
				return
			}
			t.Errorf("Unexpected pod in index: %s", pod.ID())
		})
		require.Equal(t, 2, count, "unexpected index contents")
	})
}
