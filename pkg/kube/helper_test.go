package kube

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func TestWriteResolvConf(t *testing.T) {
	tt := []struct {
		name          string
		path          string
		conf          *k8s.DNSConfig
		expectContent string
	}{
		{
			name: "only servers",
			path: os.TempDir() + "resolv.conf.test1",
			conf: &k8s.DNSConfig{
				Servers: []string{"10.0.0.12", "192.168.1.1"},
			},
			expectContent: `
nameserver 10.0.0.12
nameserver 192.168.1.1
`,
		},
		{
			name: "only searches",
			path: os.TempDir() + "resolv.conf.test2",
			conf: &k8s.DNSConfig{
				Searches: []string{"mongo.cluster.local", "mongo"},
			},
			expectContent: `
search mongo.cluster.local, mongo
`,
		},
		{
			name: "servers and searches ",
			path: os.TempDir() + "resolv.conf.test3",
			conf: &k8s.DNSConfig{
				Servers:  []string{"10.0.0.12", "192.168.1.1"},
				Searches: []string{"mongo.cluster.local", "mongo"},
			},
			expectContent: `
nameserver 10.0.0.12
nameserver 192.168.1.1
search mongo.cluster.local, mongo
`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := writeResolvConf(tc.path, tc.conf)
			require.NoError(t, err)
			actual, err := ioutil.ReadFile(tc.path)
			require.NoError(t, err)
			require.Equal(t, tc.expectContent, string(actual))
		})
	}

}
