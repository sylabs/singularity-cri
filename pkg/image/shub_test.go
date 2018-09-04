package image

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseShubRef(t *testing.T) {
	tt := []struct {
		ref         string
		expect      shubImageInfo
		expectError error
	}{
		{
			ref: "shub://vsoch/hello-world:some_tag,another_tag",
			expect: shubImageInfo{
				ref:       "shub://vsoch/hello-world:some_tag,another_tag",
				user:      "vsoch",
				container: "hello-world",
				tags:      []string{"some_tag", "another_tag"},
			},
		},
		{
			ref: "vsoch/hello-world:latest",
			expect: shubImageInfo{
				ref:       "shub://vsoch/hello-world:latest",
				user:      "vsoch",
				container: "hello-world",
				tags:      []string{"latest"},
			},
		},
		{
			ref:         "hello-world:2",
			expect:      shubImageInfo{},
			expectError: fmt.Errorf("not a valid shub reference"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.ref, func(t *testing.T) {
			actual, err := parseShubRef(tc.ref)
			require.Equal(t, tc.expectError, err)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestShubImageInfo_Filename(t *testing.T) {
	tt := []struct {
		info   shubImageInfo
		expect string
	}{
		{
			expect: "vsoch_hello-world.sif",
			info: shubImageInfo{
				ref:       "shub://vsoch/hello-world:some_tag,another_tag",
				user:      "vsoch",
				container: "hello-world",
				tags:      []string{"some_tag", "another_tag"},
			},
		},
		{
			expect: "vsoch_hello-world.sif",
			info: shubImageInfo{
				ref:       "shub://vsoch/hello-world:latest",
				user:      "vsoch",
				container: "hello-world",
				tags:      []string{"latest"},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.expect, func(t *testing.T) {
			actual := tc.info.Id()
			require.Equal(t, tc.expect, actual)
		})
	}
}
