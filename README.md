# Singularity-CRI

[![CircleCI](https://circleci.com/gh/sylabs/singularity-cri.svg?style=svg&circle-token=276de7aa1d82749ecf8ed6513c72399041885dec)](https://circleci.com/gh/sylabs/singularity-cri)
[![Code Coverage](https://codecov.io/gh/sylabs/singularity-cri/branch/master/graph/badge.svg)](https://codecov.io/gh/sylabs/singularity-cri)
[![Go Report Card](https://goreportcard.com/badge/github.com/sylabs/singularity-cri)](https://goreportcard.com/report/github.com/sylabs/singularity-cri)

This repository contains Singularity implementation of
[Kubernetes CRI](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-node/container-runtime-interface.md).
Singularity-CRI consists of two separate services: runtime and image, each of which implements 
K8s RuntimeService and ImageService respectively.

The Singularity-CRI is currently under development and passes 71/74
[validation tests](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/validation.md).
Note that used test suite is taken from `v1.13.0` tag. Detailed report can be found
[here](https://docs.google.com/spreadsheets/d/1Ym3K4LddqKNc4LCh8jr5flN7YDxfnM_hrLxpeDJRO1k/edit?usp=sharing).

## Quick start

Complete documentation can be found [here](https://sylabs.io/guides/cri/1.0/user-guide). 
Further a quick steps provided to set up Singularity-CRI from source.

In order to use Singularity-CRI install the following:

- [git](https://git-scm.com/downloads)
- [go 1.11+](https://golang.org/doc/install)
- [Singularity 3.1+ with OCI support](https://github.com/sylabs/singularity/blob/master/INSTALL.md)
- [inotify](http://man7.org/linux/man-pages/man7/inotify.7.html) for device plugin
- socat package to perform port forwarding

Since Singularity-CRI is now built with [go modules](https://github.com/golang/go/wiki/Modules)
there is no need to create standard [go workspace](https://golang.org/doc/code.html). If you still
prefer keeping source code under GOPATH make sure GO111MODULE is set. 

The following assumes you are installing Singularity-CRI from source outside GOPATH:
```bash
git clone https://github.com/sylabs/singularity-cri.git && \
cd singularity-cri && \
git checkout tags/v1.0.0-beta.4 -b v1.0.0-beta.4 && \
make && \
sudo make install
```

This will build the _sycri_ binary with CRI implementation. After installation you will find it in `/usr/local/bin`.

Singularity-CRI works with Singularity runtime directly so you need to have
`/usr/local/libexec/singularity/bin` your PATH environment variable.

To start Singularity-CRI simply run _sycri_ binary. By default it listens for requests on
`unix:///var/run/singularity.sock` and stores image files at `/var/lib/singularity`. 
This behaviour may be configured with config file, run `sycri -h` for more details.

## Contributing

Community contributions are always greatly appreciated. To start developing Singularity-CRI,
check out the [guidelines for contributing](CONTRIBUTING.md).

We also welcome contributions to our [user docs](https://github.com/sylabs/singularity-cri-userdocs).

## Support

To get help with Singularity-CRI, check out the [community Portal](https://sylabs.io/resources/community).
Also feel free to raise issues here or contact [maintainers](CONTRIBUTORS.md).

For additional support, [contact us](https://sylabs.io/contact-us) to receive more information.

## License

_Unless otherwise noted, this project is licensed under a Apache 2 license found in the [license file](LICENSE)._
