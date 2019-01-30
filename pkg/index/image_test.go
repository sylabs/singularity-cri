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
	"github.com/sylabs/cri/pkg/image"
)

func TestImageIndex(t *testing.T) {
	t.Run("smoke test", SmokeTestImageIndex)
	t.Run("advanced test", AdvancedTestImageIndex)
}

func SmokeTestImageIndex(t *testing.T) {
	indx := NewImageIndex()

	img1 := new(image.Info)
	img1.SetID("busybox")
	ref, err := image.ParseRef("library://library/default/busybox:1.29")
	require.NoError(t, err, "could not parse busybox ref")
	img1.SetRef(ref)

	img2 := new(image.Info)
	img2.SetID("nginx")
	ref, err = image.ParseRef("nginx@sha256:31b8e90a349d1fce7621f5a5a08e4fc519b634f7d3feb09d53fac9b12aa4d991")
	require.NoError(t, err, "could not parse nginx ref")
	img2.SetRef(ref)

	img3 := new(image.Info)
	img3.SetID("alpine")
	ref, err = image.ParseRef("library://library/default/alpine:3.8")
	require.NoError(t, err, "could not parse alpine ref")
	img3.SetRef(ref)

	img4 := new(image.Info)
	img4.SetID("alpine2")
	ref, err = image.ParseRef("library://library/default/alpine")
	require.NoError(t, err, "could not parse alpine ref")
	img4.SetRef(ref)

	t.Run("search empty index", func(t *testing.T) {
		found, err := indx.Find(img1.ID())
		require.Equal(t, ErrNotFound, err, "empty index didn't return ErrImageNotFound")
		require.Nil(t, found, "empty index returned image")
	})

	t.Run("add to empty index", func(t *testing.T) {
		err := indx.Add(img1)
		require.NoError(t, err)
		err = indx.Add(img2)
		require.NoError(t, err)
		err = indx.Add(img3)
		require.NoError(t, err)
	})

	t.Run("search non-empty index", func(t *testing.T) {
		found, err := indx.Find(img1.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID(), img1.ID(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Tags(), img1.Ref().Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Digests(), img1.Ref().Digests(), "index returned wrong image")

		found, err = indx.Find(img2.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID(), img2.ID(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Tags(), img2.Ref().Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Digests(), img2.Ref().Digests(), "index returned wrong image")

		found, err = indx.Find(img3.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID(), img3.ID(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Tags(), img3.Ref().Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Digests(), img3.Ref().Digests(), "index returned wrong image")

		found, err = indx.Find("nonExistentID")
		require.Equal(t, ErrNotFound, err, "empty index didn't return ErrImageNotFound")
		require.Nil(t, found, "empty index returned image")
	})

	t.Run("remove from index", func(t *testing.T) {
		err := indx.Remove(img2.Ref().Digests()[0])
		require.NoError(t, err, "could not remove image from index")

		found, err := indx.Find(img2.ID())
		require.Equal(t, ErrNotFound, err, "empty index didn't return ErrImageNotFound")
		require.Nil(t, found, "index returned unexpected image")
	})

	t.Run("ambiguous images", func(t *testing.T) {
		err := indx.Add(img4)
		require.NoError(t, err)

		found, err := indx.Find(img3.ID())
		require.Errorf(t, err, "index didn't error on ambiguous image id")
		require.Nil(t, found, "index returned wrong image")

		err = indx.Remove(img4.Ref().Tags()[0])
		require.NoError(t, err, "could not remove ambiguous image from index")

		found, err = indx.Find(img3.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID(), img3.ID(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Tags(), img3.Ref().Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Digests(), img3.Ref().Digests(), "index returned wrong image")
	})

}

func AdvancedTestImageIndex(t *testing.T) {
	indx := NewImageIndex()

	img1 := new(image.Info)
	img1.SetID("busybox")
	ref, err := image.ParseRef("library://library/default/busybox:1.29")
	require.NoError(t, err, "could not parse busybox ref")
	img1.SetRef(ref)

	img2 := new(image.Info)
	img2.SetID("busybox")
	ref, err = image.ParseRef("library://library/default/busybox:1.29")
	require.NoError(t, err, "could not parse busybox ref")
	ref.AddTags([]string{"library://library/default/busybox:latest"})
	ref.AddDigests([]string{"library://library/default/busybox:sha256.165768770ca428e9e6d8290d5672652773edf1f80d442252a0ec737ed2cc312c"})
	img2.SetRef(ref)

	img3 := new(image.Info)
	img3.SetID("busyboxNew")
	ref, err = image.ParseRef("library://library/default/busybox:latest")
	require.NoError(t, err, "could not parse busybox ref")
	img3.SetRef(ref)

	err = indx.Add(img1)
	require.NoError(t, err)

	t.Run("merge images", func(t *testing.T) {
		err := indx.Add(img2)
		require.NoError(t, err)

		found, err := indx.Find(img1.ID())
		require.NoError(t, err, "index returned unexpected error")
		require.Equal(t, found.ID(), img2.ID(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Tags(), img2.Ref().Tags(), "index returned wrong image")
		require.ElementsMatch(t, found.Ref().Digests(), img2.Ref().Digests(), "index returned wrong image")
	})

	t.Run("add overlapping image", func(t *testing.T) {
		err := indx.Add(img3)
		require.NoError(t, err)

		var count int
		indx.Iterate(func(image *image.Info) {
			count++

			if image.ID() == img2.ID() {
				require.ElementsMatch(t, image.Ref().Tags(), img1.Ref().Tags(), "index returned wrong old image tags")
				require.ElementsMatch(t, image.Ref().Digests(), img2.Ref().Digests(), "index returned wrong old image digests")
			}
			if image.ID() == img3.ID() {
				require.ElementsMatch(t, image.Ref().Tags(), img3.Ref().Tags(), "index returned wrong new image tags")
				require.ElementsMatch(t, image.Ref().Digests(), img3.Ref().Digests(), "index returned wrong new image digests")
			}
		})
		require.Equal(t, 2, count)
	})
}
