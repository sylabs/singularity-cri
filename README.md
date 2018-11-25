# Singularity CRI

[![CircleCI](https://circleci.com/gh/sylabs/cri.svg?style=svg&circle-token=276de7aa1d82749ecf8ed6513c72399041885dec)](https://circleci.com/gh/sylabs/cri)
<a href="https://app.zenhub.com/workspace/o/sylabs/cri/boards"><img src="https://raw.githubusercontent.com/ZenHubIO/support/master/zenhub-badge.png"></a>

This repository contains Singularity implementation of [Kubernetes CRI](https://github.com/kubernetes/community/blob/master/contributors/devel/container-runtime-interface.md). Singularity CRI consists of
two separate services: runtime and image, each of which implements K8s RuntimeService and ImageService respectively.


The CRI is currently under development and passes 13/71 [validation tests](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/validation.md).

## Quick Start

To work on Singularity CRI install the following:

- [git](https://git-scm.com/downloads)
- [go 1.10+](https://golang.org/doc/install)
- [dep](https://golang.github.io/dep/docs/installation.html)
- [gometalinter](https://github.com/alecthomas/gometalinter#installing)
- singularity with OCI support from https://github.com/cclerget/singularity/tree/master-oci (note the _fork_ repository and the _master-oci_ branch)
- build-essential/Development tools and libssl-dev uuid-dev squashfs-tools -- packages

Make sure you configured [go workspace](https://golang.org/doc/code.html).

To set up project do the following:

```bash
go get https://github.com/sylabs/cri
cd $GOPATH/src/github.com/sylabs/cri
make dep
```

CRI works with Singularity runtime directly so you need to have `/usr/local/libexec/singularity/bin` set up in your PATH environment variable.

After those steps you can start working on the project.

Make sure to run linters before submitting a PR:

```bash
make lint
```


## Running and testing

To build server you can use Makefile:

```bash
make build
```

This will produce the _sycri_ binary with CRI server implementation appear in a bin directory.

To start CRI server simply run _sycri_ binary. By default CRI listens for requests on
`unix:///var/run/singularity.sock` and stores image files at `/var/lib/singularity`. This behaviour may be configured
with flags, run `./sycri -h` for more details.

##
To run unit tests you can use Makefile:
```bash
sudo PATH=$PATH make test
```

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
	 ```txt 
	CONTAINER_RUNTIME_ENDPOINT=unix:///var/run/singularity.sock
	IMAGE_SERVICE_ENDPOINT=unix:///var/run/singularity.sock
	```
	For details on all options available see [`crictl install page`](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/crictl.md#install-crictl).

3. Build and launch CRI server:
	 ```bash
	(rm bin/sycri || true) && 
	make && 
	sudo PATH=$PATH ./bin/sycri
	```

4. In separate terminal run nginx pod:
	```bash
	$ cd examples
	
	$ sudo crictl runp test-pod.json
	0e0538d57a52d8673b9ad5124dd017087b3f5292f82cb9406c10f270f8f531fa
	
	$ sudo crictl pods
    POD ID              CREATED             STATE               NAME                NAMESPACE           ATTEMPT
    0e0538d57a52d       26 seconds ago      Ready               test-sandbox       default             1
	```

5. Then create & start nginx container inside just created pod:
	```bash
	$ sudo crictl pull nginx

	# sudo crictl create <podID> nginx.json test-pod.json
	$ sudo crictl create 0e0538d57a52d nginx.json test-pod.json
	7a83219a135ebb79133bac065d861e174488ba81a6622a10e2ec7e8b5b1b4371
	
	$ sudo crictl ps -a
	CONTAINER ID        IMAGE               CREATED             STATE               NAME                ATTEMPT             POD ID
    7a83219a135eb       nginx               4 seconds ago       Created             nginx-container     1                   0e0538d57a52d
	
	# sudo crictl start <containerID>
	$ sudo crictl start 7a83219a135eb
	7a83219a135eb
	```
	
6. You can also run container that outputs some system info (good to smoke test CRI):
	```bash
	$ sudo crictl pull library://sashayakovtseva/test/test-info

	# sudo crictl create <podID> nginx.json test-pod.json
	$ sudo crictl create 0e0538d57a52d info-cont.json test-pod.json
	bf040d311ca7d929ee20de4973df5c00aaf6f0e733feb695e985757686fb121b
	
	$ sudo crictl ps -a
	CONTAINER ID        IMAGE                                      CREATED             STATE               NAME                ATTEMPT             POD ID
	bf040d311ca7d       library://sashayakovtseva/test/test-info   10 seconds ago      Created             testcontainer       1                   0e0538d57a52d

	# sudo crictl start <containerID>
	$ sudo crictl start bf040d311ca7d
	bf040d311ca7d
	```
	
	If everything is fine you should see something like following in terminal with CRI running:

		...
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
		content of /proc/self/fd
				Lr-x------        0	0 -> /dev/null
				Lrwx------        0	1 -> /dev/pts/6
				Lrwx------        0	2 -> /dev/pts/6
				Lr-x------        0	3 -> 
				Lrwx------        0	4 -> anon_inode:[eventpoll]
		content of /proc/self/ns
				Lrwxrwxrwx        0	cgroup -> cgroup:[4026531835]
				Lrwxrwxrwx        0	ipc -> ipc:[4026532487]
				Lrwxrwxrwx        0	mnt -> mnt:[4026532488]
				Lrwxrwxrwx        0	net -> net:[4026532429]
				Lrwxrwxrwx        0	pid -> pid:[4026532427]
				Lrwxrwxrwx        0	pid_for_children -> pid:[4026532427]
				Lrwxrwxrwx        0	user -> user:[4026531837]
				Lrwxrwxrwx        0	uts -> uts:[4026532486]
		content of /tmp
		content of /mounted1
				-rwxr-xr-x        0	06712c8f7001b065487385e54971e0ecbe27785ad73b5834901fba0feb81402f -> 
				-rwxr-xr-x        0	0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0 -> 
				-rwxr-xr-x        0	20dfcf5b2811cacac83cf8b201f674c43a09424dc2865ca9a1c7395e670cbbfa -> 
				-rwxr-xr-x        0	2350769d4e7ba2cc7b0f59672205f54f260d09903fd354c002b782966d91dcb1 -> 
				-rwxr-xr-x        0	4f3ca194232440b204aeb0d46c1bd3502f573fa5bdd6b33847008e1ceaea1b1f -> 
				-rwxr-xr-x        0	ab7e1728de2a7e556e0d9ec7b6e98a2c0957dbd0d2bd30d56238e4ef4d7465d0 -> 
				-rw-r--r--        0	registry.json -> 
				-rw-rw-rw-        0	test-lala -> 
		read "/mounted2" error: open /mounted2: no such file or directory
		could not create file: open /mounted1/test-lala: read-only file system

7. Cleanup examples

	The quickest way to cleanup is simply pod removal:
	```bash
	# sudo crictl rmp <podID>
	$ sudo crictl rmp 0e0538d57a52d
	```

	Note: If you prefer more gentle cleanup you can stop and remove containers first and then stop and remove corresponding pod.


## Project Structure

```
- cmd/
	- server/			Singularity CRI server
- examples/				Example json configs to test CRI
- pkg/	
	- image/			Package for working with SIF images (pulling, storing,  etc...)
	- index/			Truncindex wrappers to store pods, containers and images 
	- kube/				Kubernetes specific types, e.g. pods and containers
	- namespace/			Package for manipulating linux namespaces
	- rand/				Package for generating identifiers
	- server/			CRI implementation
		- image/		Image service implementation
		- runtime/		Runtime service implementation
	- singularity/			Common for services Singularity constants
		- runtime/		Singularity runtime specific tools
	- truncindex/			General trie implementation
- vendor/				Vendored dependencies (generated by dep)
- Gopkg.lock				Generated dependency graph (generated by dep)
- Gopkg.toml				Dependency rules (used by dep)
```

## Resources

* [How to Write Go Code](https://golang.org/doc/code.html)
* [Effective Go](https://golang.org/doc/effective_go.html)
* [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
* [GitHub Flow](https://guides.github.com/introduction/flow/)
* [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
* [CRI tools](https://github.com/kubernetes-sigs/cri-tools)
