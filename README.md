# Singularity CRI

[![CircleCI](https://circleci.com/gh/sylabs/singularity-cri.svg?style=svg&circle-token=276de7aa1d82749ecf8ed6513c72399041885dec)](https://circleci.com/gh/sylabs/singularity-cri)
<a href="https://app.zenhub.com/workspace/o/sylabs/singularity-cri//boards"><img src="https://raw.githubusercontent.com/ZenHubIO/support/master/zenhub-badge.png"></a>

This repository contains Singularity implementation of [Kubernetes CRI](https://github.com/kubernetes/community/blob/master/contributors/devel/container-runtime-interface.md). Singularity CRI consists of
two separate services: runtime and image, each of which implements K8s RuntimeService and ImageService respectively.


The CRI is currently under development and passes 70/74 [validation tests](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/validation.md).
Note that used test suite is taken from master branch. Detailed report can be found [here](https://docs.google.com/spreadsheets/d/1Ym3K4LddqKNc4LCh8jr5flN7YDxfnM_hrLxpeDJRO1k/edit?usp=sharing).

## Quick Start

To work on Singularity CRI install the following:

- [git](https://git-scm.com/downloads)
- [go 1.10+](https://golang.org/doc/install)
- [dep](https://golang.github.io/dep/docs/installation.html)
- [gometalinter](https://github.com/alecthomas/gometalinter#installing)
- [singularity with OCI support](https://github.com/sylabs/singularity/blob/master/INSTALL.md)
- socat package to perform port forwarding

Make sure you configured [go workspace](https://golang.org/doc/code.html).

To set up project do the following:

```bash
go get github.com/sylabs/singularity-cri/
cd $GOPATH/src/github.com/sylabs/singularity-cri/
make dep
```

CRI works with Singularity runtime directly so you need to have `/usr/local/libexec/singularity/bin` set up in your PATH environment variable.

After those steps you can start working on the project.

Make sure to run linters before submitting a PR:

```bash
make lint
```


## Installing

To install CRI run the following:

```bash
make && sudo make install
```

This will build the _sycri_ binary with CRI server implementation and download _fakesh_ binary that is required to
run containers built from scratch. After installation you will see both binaries in
`/usr/local/bin`.


To start CRI server simply run _sycri_ binary. By default CRI listens for requests on
`unix:///var/run/singularity.sock` and stores image files at `/var/lib/singularity`. This behaviour may be configured
with flags, run `sycri -h` for more details.

##
To run unit tests you can use Makefile:
```bash
sudo make test
```

## Important notes

Because images external to the Library are in a format other than SIF, when pulled they are converted to this native
format for use by Singularity. Each time a SIF file is created through this conversion process a timestamp is
automatically generated and captured as SIF metadata. Unfortunately, changes in the timestamp result in uniquely
tagged images - even though the only difference is the timestamp in the SIF metadata. This matter has been classified
as a known issue for documentation; refer to [issue](https://github.com/sylabs/singularity-cri/issues/15) for additional details.

## Developers guide

To test CRI in interactive mode we suggest the following workflow:
 
1. Install [`crictl`](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/crictl.md):
```bash
VERSION="v1.12.0"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```

2. Configure it work with Singularity CRI. Create `/etc/crictl.yaml` config file and add the following:
```text
runtime-endpoint: unix:///var/run/singularity.sock
image-endpoint: unix:///var/run/singularity.sock
timeout: 10
debug: false
```
	For details on all options available see [`crictl install page`](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/crictl.md#install-crictl).

3. Build and launch CRI server (optional: specify desired log level with `-v` flag):
```bash
make clean &&
make && 
sudo make install &&
sudo sycri -v=10
```

4. In separate terminal run nginx pod:
```bash
$ cd examples

$ sudo crictl runp net-pod.json
0e0538d57a52d8673b9ad5124dd017087b3f5292f82cb9406c10f270f8f531fa

$ sudo crictl pods
POD ID              CREATED             STATE               NAME                NAMESPACE           ATTEMPT
0e0538d57a52d       26 seconds ago      Ready               networking          default             1
```

5. Then create & start nginx container inside the just created pod:
```bash
$ sudo crictl pull nginx

# sudo crictl create <podID> nginx.json net-pod.json
$ sudo crictl create 0e0538d57a52d nginx.json net-pod.json
7a83219a135ebb79133bac065d861e174488ba81a6622a10e2ec7e8b5b1b4371

$ sudo crictl ps -a
CONTAINER ID        IMAGE               CREATED             STATE               NAME                ATTEMPT             POD ID
7a83219a135eb       nginx               4 seconds ago       Created             nginx-container     1                   0e0538d57a52d

# sudo crictl start <containerID>
$ sudo crictl start 7a83219a135eb
7a83219a135eb
```

Verify nginx container is running by openning [localhost:80](http://localhost:80) in any browser. 
You should see nginx welcome page.
	
6. You can also run container that outputs some system info (to smoke test CRI):
```bash
$ sudo crictl pull cloud.sylabs.io/sashayakovtseva/test/test-info

# sudo crictl create <podID> info-cont.json net-pod.json
$ sudo crictl create 0e0538d57a52d info-cont.json net-pod.json
bf040d311ca7d929ee20de4973df5c00aaf6f0e733feb695e985757686fb121b

$ sudo crictl ps -a
CONTAINER ID        IMAGE                                    		  	CREATED             STATE               NAME                ATTEMPT             POD ID
bf040d311ca7d       cloud.sylabs.io/sashayakovtseva/test/test-info   	10 seconds ago      Created             testcontainer       1                   0e0538d57a52d

# sudo crictl start <containerID>
$ sudo crictl start bf040d311ca7d
bf040d311ca7d
```
	
Verify container executed correctly by opening logs:

 ```bash
# sudo crictl logs <containerID>
$ sudo crictl logs bf040d311ca7d
```

The expected output is the following:
```text
args: [./test]
mounts: 602 548 0:57 / / rw,relatime - overlay overlay rw,lowerdir=/var/run/singularity/containers/fa96e2cdaec1081a8b229fe2d8f64ac80b698b7a07f303629fb60b36abbeec8e/bundle/rootfs,upperdir=/var/run/singularity/containers/fa96e2cdaec1081a8b229fe2d8f64ac80b698b7a07f303629fb60b36abbeec8e/bundle/overlay/upper,workdir=/var/run/singularity/containers/fa96e2cdaec1081a8b229fe2d8f64ac80b698b7a07f303629fb60b36abbeec8e/bundle/overlay/work
603 602 0:50 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
604 602 0:59 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
605 604 0:60 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
606 604 0:61 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
607 604 0:49 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
608 602 0:56 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
609 602 0:22 /singularity/pods/85d02f45ee7fdf05aa199abafad6b1617fd018b3aacf30883c4724ebb025dac2/hostname /etc/hostname ro,relatime shared:5 - tmpfs tmpfs rw,size=403956k,mode=755
610 602 8:1 /var/lib/singularity /mounted1 ro,relatime - ext4 /dev/sda1 rw,errors=remount-ro,data=ordered

hostname: networking <nil>
pwd: / <nil>
content of /
	     Lrwxrwxrwx        0	.exec -> .singularity.d/actions/exec
	     Lrwxrwxrwx        0	.run -> .singularity.d/actions/run
	     Lrwxrwxrwx        0	.shell -> .singularity.d/actions/shell
	     drwxr-xr-x        0	.singularity.d -> 
	     Lrwxrwxrwx        0	.test -> .singularity.d/actions/test
	     drwxr-xr-x        0	bin -> 
	     drwxr-xr-x        0	dev -> 
	     Lrwxrwxrwx        0	environment -> .singularity.d/env/90-environment.sh
	     drwxr-xr-x        0	etc -> 
	     drwxr-xr-x        0	home -> 
	     drwxr-xr-x        0	lib -> 
	     drwxr-xr-x        0	media -> 
	     drwxr-xr-x        0	mnt -> 
	     drwxr-xr-x        0	mounted1 -> 
	     dr-xr-xr-x        0	proc -> 
	     drwx------        0	root -> 
	     drwxr-xr-x        0	run -> 
	     drwxr-xr-x        0	sbin -> 
	     Lrwxrwxrwx        0	singularity -> .singularity.d/runscript
	     drwxr-xr-x        0	srv -> 
	     dr-xr-xr-x        0	sys -> 
	     -rwxr-xr-x        0	test -> 
	    dtrwxr-xr-x        0	tmp -> 
	     drwxr-xr-x        0	usr -> 
	     drwxr-xr-x        0	var -> 
uid=0 gid=0 euid=0 egid=0
pid=30 ppid=0
envs=[LD_LIBRARY_PATH=/.singularity.d/libs SHLVL=1 MY_ANOTHER_VAR=is-awesome PS1=Singularity>  TERM=xterm PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin PWD=/ MY_CUSTOM_VAR=singularity-cri]
...
```

7. Cleanup examples
	The quickest way to cleanup is simply pod removal:
```bash
# sudo crictl stopp <podID>
$ sudo crictl stopp 0e0538d57a52d

# sudo crictl rmp <podID>
$ sudo crictl rmp 0e0538d57a52d
```

## Resources

* [How to Write Go Code](https://golang.org/doc/code.html)
* [Effective Go](https://golang.org/doc/effective_go.html)
* [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
* [GitHub Flow](https://guides.github.com/introduction/flow/)
* [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
* [CRI tools](https://github.com/kubernetes-sigs/cri-tools)
