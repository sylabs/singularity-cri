package fs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMountPoint(t *testing.T) {
	tt := []struct {
		path   string
		parent string
		err    error
	}{
		{
			path: "/proc_",
			err:  fmt.Errorf("could not resolve path /proc_: lstat /proc_: no such file or directory"),
		},
		{
			path:   "/proc",
			parent: "/proc",
		},
		{
			path:   "/home",
			parent: "/",
		},
		{
			path:   "/dev/null",
			parent: "/dev",
		},
		{
			path:   "/var/run/mount",
			parent: "/run",
		},
		{
			path:   "/var/lib",
			parent: "/",
		},
		{
			path:   "/proc/self",
			parent: "/proc",
		},
		{
			path: "/proc/fake",
			err:  fmt.Errorf("could not resolve path /proc/fake: lstat /proc/fake: no such file or directory"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.path, func(t *testing.T) {
			parent, err := MountPoint(tc.path)
			require.Equal(t, tc.parent, parent)
			require.Equal(t, tc.err, err)
		})
	}
}
