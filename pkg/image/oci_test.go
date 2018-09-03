package image

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOCIRef(t *testing.T) {
	tt := []struct {
		ref    string
		expect ociImageInfo
	}{
		{
			ref: "docker://ubuntu:some_tag,another_tag",
			expect: ociImageInfo{
				ref:       "docker://ubuntu:some_tag,another_tag",
				container: "ubuntu",
				tags:      []string{"some_tag", "another_tag"},
			},
		},
		{
			ref: "docker://sylabs/ubuntu",
			expect: ociImageInfo{
				ref:       "docker://sylabs/ubuntu",
				repo:      "sylabs",
				container: "ubuntu",
				tags:      []string{"latest"},
			},
		},
		{
			ref: "ubuntu:16.4",
			expect: ociImageInfo{
				ref:       "docker://ubuntu:16.4",
				container: "ubuntu",
				tags:      []string{"16.4"},
			},
		},
		{
			ref: "ubuntu",
			expect: ociImageInfo{
				ref:       "docker://ubuntu",
				container: "ubuntu",
				tags:      []string{"latest"},
			},
		},
		{
			ref: "docker://gcr.io/cri-tools/test-image-tags:2",
			expect: ociImageInfo{
				ref:       "docker://gcr.io/cri-tools/test-image-tags:2",
				domain:    "gcr.io",
				repo:      "cri-tools",
				container: "test-image-tags",
				tags:      []string{"2"},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.ref, func(t *testing.T) {
			actual := parseOCIRef(tc.ref)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestOCIImageInfo_Filename(t *testing.T) {
	tt := []struct {
		info   ociImageInfo
		expect string
	}{
		{
			expect: "ubuntu.sif",
			info: ociImageInfo{
				ref:       "docker://ubuntu:some_tag,another_tag",
				container: "ubuntu",
				tags:      []string{"some_tag", "another_tag"},
			},
		},
		{
			expect: "sylabs_ubuntu.sif",
			info: ociImageInfo{
				ref:       "docker://sylabs:ubuntu",
				repo:      "sylabs",
				container: "ubuntu",
				tags:      []string{"latest"},
			},
		},
		{
			expect: "ubuntu.sif",
			info: ociImageInfo{
				ref:       "docker://ubuntu:16.4",
				container: "ubuntu",
				tags:      []string{"16.4"},
			},
		},
		{
			expect: "ubuntu.sif",
			info: ociImageInfo{
				ref:       "docker://ubuntu",
				container: "ubuntu",
				tags:      []string{"latest"},
			},
		},
		{
			expect: "gcr.io_cri-tools_test-image-tags.sif",
			info: ociImageInfo{
				ref:       "docker://gcr.io/cri-tools/test-image-tags:2",
				domain:    "gcr.io",
				repo:      "cri-tools",
				container: "test-image-tags",
				tags:      []string{"2"},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.expect, func(t *testing.T) {
			actual := tc.info.Filename()
			require.Equal(t, tc.expect, actual)
		})
	}
}
