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

package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBuildConfig(t *testing.T) {
	tt := []struct {
		name   string
		in     []byte
		expect BuildConfig
	}{
		{
			name:   "no output",
			in:     nil,
			expect: BuildConfig{},
		},
		{
			name: "no confdir",
			in: []byte(`
PACKAGE_NAME=singularity
PACKAGE_VERSION=3.1.0-354.g3bc381b61
PREFIX=/usr/local
EXECPREFIX=/usr/local
LIBDIR=/usr/local/lib
LOCALEDIR=/usr/local/share/locale
MANDIR=/usr/local/share/man
SESSIONDIR=/usr/local/var/singularity/mnt/session
`),
			expect: BuildConfig{},
		},
		{
			name: "with confdir",
			in: []byte(`
PACKAGE_NAME=singularity
PACKAGE_VERSION=3.1.0-354.g3bc381b61
PREFIX=/usr/local
EXECPREFIX=/usr/local
LIBDIR=/usr/local/lib
LOCALEDIR=/usr/local/share/locale
MANDIR=/usr/local/share/man
SESSIONDIR=/usr/local/var/singularity/mnt/session
SINGULARITY_CONFDIR=/usr/local/etc/singularity
`),
			expect: BuildConfig{
				SingularityConfdir: "/usr/local/etc/singularity",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := parseBuildConfig(tc.in)
			require.Equal(t, tc.expect, actual)
		})
	}
}
