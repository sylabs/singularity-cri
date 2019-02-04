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

package slice

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeString(t *testing.T) {
	tt := []struct {
		name   string
		s1     []string
		s2     []string
		expect []string
	}{
		{
			name:   "no intersection",
			s1:     []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
			s2:     []string{"gcr.io/cri-tools/test-image-tags:3"},
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
		},
		{
			name:   "intersection",
			s1:     []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
			s2:     []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:3"},
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := MergeString(tc.s1, tc.s2...)
			require.ElementsMatch(t, tc.expect, actual)
		})
	}
}

func TestRemoveFromString(t *testing.T) {
	tt := []struct {
		name   string
		s      []string
		v      string
		expect []string
	}{
		{
			name:   "not found",
			s:      []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
			v:      "gcr.io/cri-tools/test-image-tags:3",
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
		},
		{
			name:   "single occurrence",
			s:      []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
			v:      "gcr.io/cri-tools/test-image-tags:2",
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:3"},
		},
		{
			name:   "multiple occurrence",
			s:      []string{"gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
			v:      "gcr.io/cri-tools/test-image-tags:2",
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := RemoveFromString(tc.s, tc.v)
			require.Equal(t, tc.expect, actual)
		})
	}
}
