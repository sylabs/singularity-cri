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
			actual := MergeString(tc.s1, tc.s2)
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
