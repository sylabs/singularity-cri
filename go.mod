module github.com/sylabs/singularity-cri

go 1.11

require (
	github.com/NVIDIA/gpu-monitoring-tools v0.0.0-20190227022151-81c885550fa1
	github.com/containerd/cgroups v0.0.0-20181219155423-39b18af02c41
	github.com/containernetworking/cni v0.6.0
	github.com/containernetworking/plugins v0.7.4 // indirect
	github.com/containers/storage v0.0.0-20181207174215-bf48aa83089d // indirect
	github.com/coreos/go-iptables v0.4.0 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/emicklei/go-restful v2.8.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.4.7
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/kr/pty v1.1.3
	github.com/kubernetes-sigs/cri-o v1.12.3
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc6
	github.com/opencontainers/runtime-spec v0.0.0-20180913141938-5806c3563733
	github.com/opencontainers/runtime-tools v0.8.0
	github.com/opencontainers/selinux v1.0.0-rc1
	github.com/sirupsen/logrus v1.2.0 // indirect
	github.com/stretchr/testify v1.2.2
	github.com/sylabs/scs-library-client v0.1.0
	github.com/sylabs/sif v1.0.3
	github.com/sylabs/singularity v0.0.0-20190527133303-3b81f6571f4e
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	github.com/tchap/go-patricia v2.2.6+incompatible
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20180816142147-da425ebb7609 // indirect
	golang.org/x/sys v0.0.0-20190321052220-f7bb7a8bee54
	google.golang.org/genproto v0.0.0-20181109154231-b5d43981345b // indirect
	google.golang.org/grpc v1.20.0
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20181121071145-b7bd5f2d334c // indirect
	k8s.io/apimachinery v0.0.0-20181126123124-70adfbae261e // indirect
	k8s.io/apiserver v0.0.0-20181121231732-e3c8fa95bba5 // indirect
	k8s.io/client-go v0.0.0-20181010045704-56e7a63b5e38
	k8s.io/klog v0.2.0 // indirect
	k8s.io/kubernetes v1.12.5
	k8s.io/utils v0.0.0-20181115163542-0d26856f57b3 // indirect
)

replace golang.org/x/crypto => github.com/sylabs/golang-x-crypto v0.0.0-20181006204705-4bce89e8e9a9
