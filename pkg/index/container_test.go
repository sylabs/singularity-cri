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

func TestContainerIndex(t *testing.T) {
	indx := NewContainerIndex()

	busybox := kube.NewContainer(nil, nil, nil, "")
	nginx := kube.NewContainer(nil, nil, nil, "")
	alpine := kube.NewContainer(nil, nil, nil, "")

	t.Run("empty index", func(t *testing.T) {
		found, err := indx.Find(busybox.ID())
		require.Equal(t, ErrNotFound, err, "empty index didn't return ErrNotFound")
		require.Nil(t, found, "empty index returned container")

		found, err = indx.Find("illegal id")
		require.EqualError(t, err, "could not search index: illegal character: ' '",
			"illegal id didn't lead to error")
		require.Nil(t, found, "illegal id returned container")

		err = indx.Remove(busybox.ID())
		require.NoError(t, err, "empty index returned error on remove")
		err = indx.Remove("illegal id")
		require.EqualError(t, err, "could not remove container: illegal character: ' '",
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
		require.Equal(t, found, busybox, "index returned wrong container")

		found, err = indx.Find(nginx.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found, nginx, "index returned wrong container")

		found, err = indx.Find(alpine.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found, alpine, "index returned wrong container")

		found, err = indx.Find("nonExistentID")
		require.Equal(t, ErrNotFound, err, "index didn't return ErrNotFound")
		require.Nil(t, found, "index returned unexpected container")
	})

	t.Run("remove from index", func(t *testing.T) {
		err := indx.Remove(nginx.ID())
		require.NoError(t, err, "could not remove container from index")

		found, err := indx.Find(nginx.ID())
		require.Equal(t, ErrNotFound, err, "index didn't return ErrNotFound")
		require.Nil(t, found, "removed container is still returned")

		err = indx.Remove(nginx.ID())
		require.NoError(t, err, "removing removed container lead to error")
	})

	t.Run("iterate containers", func(t *testing.T) {
		var count int
		indx.Iterate(func(cont *kube.Container) {
			count++
			if cont.ID() == busybox.ID() ||
				cont.ID() == alpine.ID() {
				return
			}
			t.Errorf("Unexpected container in index: %s", cont.ID())
		})
		require.Equal(t, 2, count, "unexpected index contents")
	})
}
