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
	"github.com/sylabs/singularity-cri/pkg/image"
)

func TestImageIndex(t *testing.T) {
	t.Run("smoke test", SmokeTestImageIndex)
	t.Run("advanced test", AdvancedTestImageIndex)
}

func SmokeTestImageIndex(t *testing.T) {
	indx := NewImageIndex()

	ref, err := image.ParseRef("library://library/default/busybox:1.29")
	require.NoError(t, err, "could not parse busybox ref")
	busybox := &image.Info{
		ID:  "busybox",
		Ref: ref,
	}

	ref, err = image.ParseRef("nginx@sha256:31b8e90a349d1fce7621f5a5a08e4fc519b634f7d3feb09d53fac9b12aa4d991")
	require.NoError(t, err, "could not parse nginx ref")
	nginx := &image.Info{
		ID:  "nginx",
		Ref: ref,
	}

	ref, err = image.ParseRef("library://library/default/alpine:3.8")
	require.NoError(t, err, "could not parse alpine ref")
	alpine := &image.Info{
		ID:  "alpine",
		Ref: ref,
	}

	ref, err = image.ParseRef("library://library/default/alpine")
	require.NoError(t, err, "could not parse alpine ref")
	alpine2 := &image.Info{
		ID:  "alpine2",
		Ref: ref,
	}

	t.Run("search empty index", func(t *testing.T) {
		found, err := indx.Find(busybox.ID)
		require.Equal(t, ErrNotFound, err, "empty index didn't return ErrNotFound")
		require.Nil(t, found, "empty index returned image")
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
		found, err := indx.Find(busybox.ID)
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID, busybox.ID, "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Tags(), busybox.Ref.Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Digests(), busybox.Ref.Digests(), "index returned wrong image")

		found, err = indx.Find(nginx.ID)
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID, nginx.ID, "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Tags(), nginx.Ref.Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Digests(), nginx.Ref.Digests(), "index returned wrong image")

		found, err = indx.Find(alpine.ID)
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID, alpine.ID, "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Tags(), alpine.Ref.Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Digests(), alpine.Ref.Digests(), "index returned wrong image")

		found, err = indx.Find("nonExistentID")
		require.Equal(t, ErrNotFound, err, "index didn't return ErrNotFound")
		require.Nil(t, found, "index returned unexpected image")
	})

	t.Run("remove from index", func(t *testing.T) {
		err := indx.Remove(nginx.Ref.Digests()[0])
		require.NoError(t, err, "could not remove image from index")

		found, err := indx.Find(nginx.ID)
		require.Equal(t, ErrNotFound, err, "index didn't return ErrNotFound")
		require.Nil(t, found, "removed image is still returned")
	})

	t.Run("ambiguous images", func(t *testing.T) {
		err := indx.Add(alpine2)
		require.NoError(t, err)

		alpine.Ref.AddDigests([]string{"library://library/default/alpine:sha256.somefakesha"})
		err = indx.Add(alpine)
		require.EqualError(t, err,
			"could not find old image: could not search index: multiple items found for provided prefix: alpine",
			"index updated image even when ambiguous")

		found, err := indx.Find(alpine.ID)
		require.EqualError(t, err, "could not search index: multiple items found for provided prefix: alpine",
			"index didn't error on ambiguous image id")
		require.Nil(t, found, "index returned wrong image")

		err = indx.Remove(alpine.ID)
		require.EqualError(t, err, "could not search index: multiple items found for provided prefix: alpine",
			"index didn't error on ambiguous image id")

		err = indx.Remove(alpine2.Ref.Tags()[0])
		require.NoError(t, err, "could not remove ambiguous image from index")

		alpine.Ref.AddDigests([]string{"library://library/default/alpine:sha256.somefakesha"})
		err = indx.Add(alpine)
		require.NoError(t, err, "could not update image after ambiguous removed")

		found, err = indx.Find(alpine.ID)
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID, alpine.ID, "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Tags(), alpine.Ref.Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Digests(), alpine.Ref.Digests(), "index returned wrong image")
	})

}

func AdvancedTestImageIndex(t *testing.T) {
	indx := NewImageIndex()

	ref, err := image.ParseRef("library://library/default/busybox:1.29")
	require.NoError(t, err, "could not parse busybox ref")
	busybox := &image.Info{
		ID:  "busybox",
		Ref: ref,
	}

	ref, err = image.ParseRef("library://library/default/busybox:1.29")
	require.NoError(t, err, "could not parse busybox ref")
	ref.AddTags([]string{"library://library/default/busybox:latest"})
	ref.AddDigests([]string{"library://library/default/busybox:sha256.165768770ca428e9e6d8290d5672652773edf1f80d442252a0ec737ed2cc312c"})
	updBusybox := &image.Info{
		ID:  "busybox",
		Ref: ref,
	}

	ref, err = image.ParseRef("library://library/default/busybox:latest")
	require.NoError(t, err, "could not parse busybox ref")
	busyboxNew := &image.Info{
		ID:  "busyboxNew",
		Ref: ref,
	}

	err = indx.Add(busybox)
	require.NoError(t, err)

	t.Run("merge images", func(t *testing.T) {
		err := indx.Add(updBusybox)
		require.NoError(t, err)

		found, err := indx.Find(busybox.ID)
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID, updBusybox.ID, "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Tags(), updBusybox.Ref.Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref.Digests(), updBusybox.Ref.Digests(), "index returned wrong image")
	})

	t.Run("add overlapping image", func(t *testing.T) {
		err := indx.Add(busyboxNew)
		require.NoError(t, err)

		var count int
		indx.Iterate(func(image *image.Info) {
			count++

			if image.ID == updBusybox.ID {
				require.ElementsMatch(t, image.Ref.Tags(), busybox.Ref.Tags(), "index returned wrong old image tags")
				require.ElementsMatch(t, image.Ref.Digests(), updBusybox.Ref.Digests(), "index returned wrong old image digests")
			}
			if image.ID == busyboxNew.ID {
				require.ElementsMatch(t, image.Ref.Tags(), busyboxNew.Ref.Tags(), "index returned wrong new image tags")
				require.ElementsMatch(t, image.Ref.Digests(), busyboxNew.Ref.Digests(), "index returned wrong new image digests")
			}
		})
		require.Equal(t, 2, count)
	})
}
