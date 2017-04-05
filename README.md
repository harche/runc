## runvm

`runvm` (pronounced as `run vm`) is a CLI tool for spawning and running virtual machines using docker images. runvm 
allows you to use vastly available docker images and execute the code inside a secure virtual machine using 
standard OCI compliant docker runtime. 

## Building

`runvm` currently supports the Linux platform with various architecture support. For the purpose 
of proof of concept, it only supports launching virtual machines using KVM for now.

It must be built with Go version 1.6 or higher in order for some features to function properly.

In order to enable seccomp support you will need to install `libseccomp` on your platform.
> e.g. `libseccomp-devel` for CentOS, or `libseccomp-dev` for Ubuntu

Otherwise, if you do not want to build `runvm` with seccomp support you can add `BUILDTAGS=""` when running make.

```bash
# create a 'github.com/harche' in your GOPATH/src
cd github.com/harche
git clone -b runvm https://github.com/harche/runvm.git

cd runvm

make
sudo make install
```

`runvm` will be installed to `/usr/local/sbin/runvm` on your system.



## Using runvm

This release supports launching virtual machines using KVM.

### Prerequisites

Ubuntu
```
apt-get install genisoimage
apt-get install qeum-kvm
apt-get install libvirt-bin
apt-get install qemu-system 
apt-get install qemu-system-<arch>
apt-get install qemu-utils
```

Fedora
```
yum install genisoimage
yum install qemu-kvm
yum install qemu-img
yum install qemu-kvm-tools
yum install libvirt-devel
```

Download virtual machine image from,
```
$ sudo cd /var/lib/libvirt/images   
$ sudo wget https://dl.fedoraproject.org/pub/archive/fedora/linux/releases/22/Cloud/x86_64/Images/Fedora-Cloud-Base-22-20150521.x86_64.qcow2
$ sudo mv Fedora-Cloud-Base-22-20150521.x86_64.qcow2 disk.img.orig
```
Save this in `/var/lib/libvirt/images/disk.img.orig`


### Using runvm with docker

Assuming that you have already built the `runvm` from **Building** section above, you will have to let docker 
use this new runtime so that when docker is trying to provision new container `runvm` can lanuch a virtual 
machine instead with the given docker image.

Stop the docker deamon if it's already running,
```
service docker stop
```

Execute the following command to let docker daemon know about `runvm`,
```
dockerd --add-runtime runvm=/usr/local/sbin/runvm
```

Open another shell to launch some virtual machines using docker images!

```
$ docker  run  --runtime=runvm busybox hostname
f2c647640c751414d9db7a4dffdfcf410976df2c43b7b25fed22ba41f2dd0b24
$ 
```
In above example, the command `hostname` was executed inside of a virtual machine.

```
$ sudo virsh list --all 
 Id    Name                           State
----------------------------------------------------
 116   f2c647640c751414d9db7a4dffdfcf410976df2c43b7b25fed22ba41f2dd0b24 running
$ 
```
Once the given command has completed it's execution the virtual machine is cleared 
from the system.

Note that in case you need to launch regular `cgroups` based containers all you have 
to do is to let docker use the built-in runtime `runc` that it ships with,

```
$ docker run busybox hostname
800f4cc7a69eec659b74a96c9f03165d97374b13658dfc23b885cabd3208e628
$ 
```
Docker deamon restart is *NOT* required for launching containers simultaenously using
`runc` and `runvm`


### Current Limitations
1. No Interactive shell support

### Running the test suite

`runvm` currently supports running its test suite via Docker.
To run the suite just type `make test`.

```bash
make test
```

There are additional make targets for running the tests outside of a container but this is not recommended as the tests are written with the expectation that they can write and remove anywhere.

You can run a specific test case by setting the `TESTFLAGS` variable.

```bash
# make test TESTFLAGS="-run=SomeTestFunction"
```
