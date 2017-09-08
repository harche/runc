## runvm 

`runvm` (pronounced as `run vm`) is a container runtime for spawning and running virtual machines using docker images. runvm 
allows the use of vastly available docker images and execute the code inside a virtual machine using 
standard OCI compliant docker runtime. 

`runvm` is aimed at achieving better isolation for the application running inside containers using Qemu. Better isolation is useful for applications which require the agility of containers but strong isolation provided by virtual machines, such as smart contract execution by blockchain (such as Hyperledger). 


## Building

`runvm` currently supports the Linux platform with various architecture support. For the purpose 
of proof of concept, it only supports launching virtual machines using KVM for now.

It must be built with Go version 1.6 or higher in order for some features to function properly.

In order to enable seccomp support you will need to install `libseccomp` on your platform.
> e.g. `libseccomp-devel` for CentOS, or `libseccomp-dev` for Ubuntu

Otherwise, if you do not want to build `runvm` with seccomp support you can add `BUILDTAGS=""` when running make. For docker version =< 1.12 add `apparmor` to `BUILDTAGS`. 

```bash
mkdir -p $GOPATH/src/github.com/harche
cd github.com/harche
git clone https://github.com/harche/runvm.git

cd runvm
cd Godeps/_workspace/src/github.com/libvirt/
git clone https://github.com/libvirt/libvirt-go 
$GOPATH/src/github.com/harche/runvm

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
apt-get install libvirt-dev
apt-get install qemu-system 
apt-get install qemu-system-<arch>
apt-get install qemu-utils
apt-get install util-linux
```

Fedora
```
yum install genisoimage
yum install qemu-kvm
yum install qemu-img
yum install qemu-kvm-tools
yum install libvirt-devel
yum install util-linux
```

Download virtual machine image from,
```
$ sudo cd /var/lib/libvirt/images   
$ sudo wget https://cloud-images.ubuntu.com/xenial/current/xenial-server-cloudimg-amd64-disk1.img
$ sudo mv xenial-server-cloudimg-amd64-disk1.img disk.img.orig
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
2. Docker exec won't work as no new processes are allowed to start inside a running VM
3. Docker attach won't work.


```
