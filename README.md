## runvm

`runvm` (pronounced as `run vm`) is a CLI tool for spawning and running virtual machines using docker images. runvm 
allows you to use vastly available docker images and execute the code inside a secure virtual machine using 
standard OCI compliant docker runtime. 

## Building

`runvm` currently supports the Linux platform with various architecture support. For the purpose 
of proof of concept, it only supports launching virtual machines using KVM. But we plan support `plug and play` any hypervisor in the future.

It must be built with Go version 1.6 or higher in order for some features to function properly.

In order to enable seccomp support you will need to install `libseccomp` on your platform.
> e.g. `libseccomp-devel` for CentOS, or `libseccomp-dev` for Ubuntu

Otherwise, if you do not want to build `runvm` with seccomp support you can add `BUILDTAGS=""` when running make.

```bash
# create a 'github.com/harche' in your GOPATH/src
cd github.com/harche
git clone https://github.com/harche/runvm
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
```

Fedora
```
yum install genisoimage
yum install qemu-kvm qemu-img
```

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

Let's launch some virtual machines using docker images!

```
$ docker  run  --runtime=runvm busybox hostname
f2c647640c751414d9db7a4dffdfcf410976df2c43b7b25fed22ba41f2dd0b24
$ 
```
In above example, the command `hostname` was executed inside of a virtual machine. 

Note that in case you need to launch regular `cgroups` based containers all you have 
to do is to let docker use the built-in runtime `runc` that it ships with,

```
$ docker run busybox hostname
f2c647640c751414d9db7a4dffdfcf410976df2c43b7b25fed22ba41f2dd0b24
$ 
```



### Creating an OCI Bundle

In order to use runvm you must have your container in the format of an OCI bundle.
If you have Docker installed you can use its `export` method to acquire a root filesystem from an existing Docker container.

```bash
# create the top most bundle directory
mkdir /mycontainer
cd /mycontainer

# create the rootfs directory
mkdir rootfs

# export busybox via Docker into the rootfs directory
docker export $(docker create busybox) | tar -C rootfs -xvf -
```

After a root filesystem is populated you just generate a spec in the format of a `config.json` file inside your bundle.
`runvm` provides a `spec` command to generate a base template spec that you are then able to edit.
To find features and documentation for fields in the spec please refer to the [specs](https://github.com/opencontainers/runtime-spec) repository.

```bash
runvm spec
```

### Running Virtual Machines

Assuming you have an OCI bundle from the previous step you can execute the container in two different ways.

The first way is to use the convenience command `run` that will handle creating, starting, and deleting the container after it exits.

```bash
cd /mycontainer

runvm run mycontainerid
```

If you used the unmodified `runvm spec` template this should give you a `sh` session inside the container.

The second way to start a container is using the specs lifecycle operations.
This gives you more power over how the container is created and managed while it is running.
This will also launch the container in the background so you will have to edit the `config.json` to remove the `terminal` setting for the simple examples here.
Your process field in the `config.json` should look like this below with `"terminal": false` and `"args": ["sleep", "5"]`.


```json
        "process": {
                "terminal": false,
                "user": {
                        "uid": 0,
                        "gid": 0
                },
                "args": [
                        "sleep", "5"
                ],
                "env": [
                        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                        "TERM=xterm"
                ],
                "cwd": "/",
                "capabilities": [
                        "CAP_AUDIT_WRITE",
                        "CAP_KILL",
                        "CAP_NET_BIND_SERVICE"
                ],
                "rlimits": [
                        {
                                "type": "RLIMIT_NOFILE",
                                "hard": 1024,
                                "soft": 1024
                        }
                ],
                "noNewPrivileges": true
        },
```

Now we can go though the lifecycle operations in your shell.


```bash
cd /mycontainer

runvm create mycontainerid

# view the container is created and in the "created" state
runvm list

# start the process inside the container
runvm start mycontainerid

# after 5 seconds view that the container has exited and is now in the stopped state
runvm list

# now delete the container
runvm delete mycontainerid
```

This adds more complexity but allows higher level systems to manage runvm and provides points in the containers creation to setup various settings after the container has created and/or before it is deleted.
This is commonly used to setup the container's network stack after `create` but before `start` where the user's defined process will be running.

#### Supervisors

`runvm` can be used with process supervisors and init systems to ensure that containers are restarted when they exit.
An example systemd unit file looks something like this.

```systemd
[Unit]
Description=Start My Container

[Service]
Type=forking
ExecStart=/usr/local/sbin/runvm run -d --pid-file /run/mycontainerid.pid mycontainerid
ExecStopPost=/usr/local/sbin/runvm delete mycontainerid
WorkingDirectory=/mycontainer
PIDFile=/run/mycontainerid.pid

[Install]
WantedBy=multi-user.target
```
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
