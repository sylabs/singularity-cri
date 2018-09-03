package image

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLibraryRef(t *testing.T) {
	tt := []struct {
		ref    string
		expect libraryImageInfo
	}{
		{
			ref: "library://godlovedc/ubuntu/lolcow:some_tag,another_tag",
			expect: libraryImageInfo{
				ref:        "library://godlovedc/ubuntu/lolcow:some_tag,another_tag",
				user:       "godlovedc",
				collection: "ubuntu",
				container:  "lolcow",
				tags:       []string{"some_tag", "another_tag"},
			},
		},
		{
			ref: "ubuntu/lolcow",
			expect: libraryImageInfo{
				ref:        "library://ubuntu/lolcow",
				user:       "",
				collection: "ubuntu",
				container:  "lolcow",
				tags:       []string{"latest"},
			},
		},
		{
			ref: "lolcow:2",
			expect: libraryImageInfo{
				ref:        "library://lolcow:2",
				user:       "",
				collection: "",
				container:  "lolcow",
				tags:       []string{"2"},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.ref, func(t *testing.T) {
			actual := parseLibraryRef(tc.ref)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestLibraryImageInfo_Filename(t *testing.T) {
	tt := []struct {
		info   libraryImageInfo
		expect string
	}{
		{
			expect: "godlovedc_ubuntu_lolcow.sif",
			info: libraryImageInfo{
				ref:        "library://godlovedc/ubuntu/lolcow:some_tag,another_tag",
				user:       "godlovedc",
				collection: "ubuntu",
				container:  "lolcow",
				tags:       []string{"some_tag", "another_tag"},
			},
		},
		{
			expect: "ubuntu_lolcow.sif",
			info: libraryImageInfo{
				ref:        "library://ubuntu/lolcow",
				user:       "",
				collection: "ubuntu",
				container:  "lolcow",
				tags:       []string{"latest"},
			},
		},
		{
			expect: "lolcow.sif",
			info: libraryImageInfo{
				ref:        "library://lolcow:2",
				user:       "",
				collection: "",
				container:  "lolcow",
				tags:       []string{"2"},
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
