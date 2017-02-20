// +build linux

package main

import (
	"bufio"

	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	//	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	libvirt "github.com/libvirt/libvirt-go"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
	//	"github.com/vishvananda/netns"
)

var errEmptyID = errors.New("container id cannot be empty")

var container libcontainer.Container

// loadFactory returns the configured factory instance for execing containers.
func loadFactory(context *cli.Context) (libcontainer.Factory, error) {
	root := context.GlobalString("root")
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	cgroupManager := libcontainer.Cgroupfs
	if context.GlobalBool("systemd-cgroup") {
		if systemd.UseSystemd() {
			cgroupManager = libcontainer.SystemdCgroups
		} else {
			return nil, fmt.Errorf("systemd cgroup flag passed, but systemd support for managing cgroups is not available")
		}
	}
	return libcontainer.New(abs, cgroupManager, libcontainer.CriuPath(context.GlobalString("criu")))
}

// getContainer returns the specified container instance by loading it from state
// with the default factory.
func getContainer(context *cli.Context) (libcontainer.Container, error) {
	id := context.Args().First()
	if id == "" {
		return nil, errEmptyID
	}
	factory, err := loadFactory(context)
	if err != nil {
		return nil, err
	}
	return factory.Load(id)
}

func fatalf(t string, v ...interface{}) {
	fatal(fmt.Errorf(t, v...))
}

func getDefaultImagePath(context *cli.Context) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(cwd, "checkpoint")
}

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(p specs.Process) (*libcontainer.Process, error) {
	lp := &libcontainer.Process{
		Args: p.Args,
		Env:  p.Env,
		// TODO: fix libcontainer's API to better support uid/gid in a typesafe way.
		User:            fmt.Sprintf("%d:%d", p.User.UID, p.User.GID),
		Cwd:             p.Cwd,
		Capabilities:    p.Capabilities,
		Label:           p.SelinuxLabel,
		NoNewPrivileges: &p.NoNewPrivileges,
		AppArmorProfile: p.ApparmorProfile,
	}
	for _, gid := range p.User.AdditionalGids {
		lp.AdditionalGroups = append(lp.AdditionalGroups, strconv.FormatUint(uint64(gid), 10))
	}
	for _, rlimit := range p.Rlimits {
		rl, err := createLibContainerRlimit(rlimit)
		if err != nil {
			return nil, err
		}
		lp.Rlimits = append(lp.Rlimits, rl)
	}
	return lp, nil
}

func dupStdio(process *libcontainer.Process, rootuid, rootgid int) error {
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	for _, fd := range []uintptr{
		os.Stdin.Fd(),
		os.Stdout.Fd(),
		os.Stderr.Fd(),
	} {
		if err := syscall.Fchown(int(fd), rootuid, rootgid); err != nil {
			return err
		}
	}
	return nil
}

// If systemd is supporting sd_notify protocol, this function will add support
// for sd_notify protocol from within the container.
func setupSdNotify(spec *specs.Spec, notifySocket string) {
	spec.Mounts = append(spec.Mounts, specs.Mount{Destination: notifySocket, Type: "bind", Source: notifySocket, Options: []string{"bind"}})
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("NOTIFY_SOCKET=%s", notifySocket))
}

func destroy(container libcontainer.Container) {
	if err := container.Destroy(); err != nil {
		logrus.Error(err)
	}
}

// setupIO sets the proper IO on the process depending on the configuration
// If there is a nil error then there must be a non nil tty returned
func setupIO(process *libcontainer.Process, rootuid, rootgid int, console string, createTTY, detach bool) (*tty, error) {
	// detach and createTty will not work unless a console path is passed
	// so error out here before changing any terminal settings
	if createTTY && detach && console == "" {
		return nil, fmt.Errorf("cannot allocate tty if runc will detach")
	}
	if createTTY {
		return createTty(process, rootuid, rootgid, console)
	}
	if detach {
		if err := dupStdio(process, rootuid, rootgid); err != nil {
			return nil, err
		}
		return &tty{}, nil
	}
	return createStdioPipes(process, rootuid, rootgid)
}

// createPidFile creates a file with the processes pid inside it atomically
// it creates a temp file with the paths filename + '.' infront of it
// then renames the file
func createPidFile(path string, process *libcontainer.Process) error {
	pid, err := process.Pid()
	if err != nil {
		return err
	}
	var (
		tmpDir  = filepath.Dir(path)
		tmpName = filepath.Join(tmpDir, fmt.Sprintf(".%s", filepath.Base(path)))
	)
	f, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d", pid)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func createContainer(context *cli.Context, id string, spec *specs.Spec) (libcontainer.Container, error) {
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       id,
		UseSystemdCgroup: context.GlobalBool("systemd-cgroup"),
		NoPivotRoot:      context.Bool("no-pivot"),
		NoNewKeyring:     context.Bool("no-new-keyring"),
		Spec:             spec,
	})
	if err != nil {
		return nil, err
	}

	factory, err := loadFactory(context)
	if err != nil {
		return nil, err
	}
	return factory.Create(id, config)
}

type runner struct {
	enableSubreaper bool
	shouldDestroy   bool
	detach          bool
	listenFDs       []*os.File
	pidFile         string
	console         string
	container       libcontainer.Container
	create          bool
	qemuDirectory   string
	args            []string
	netInfo         netinfo
}

func (r *runner) CreateDeltaDiskImage(deltaDiskDirectory, diskPath string) (string, error) {
	deltaImagePath, err := exec.LookPath("qemu-img")
	if err != nil {
		return "", fmt.Errorf("qemu-img is not installed on your PATH. Please, install it to run isolated qemu container")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not determine the current directory")
	}

	err = os.Chdir(deltaDiskDirectory)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", deltaDiskDirectory)
	}

	err = exec.Command(deltaImagePath, "create", "-f", "qcow2", "-b", diskPath, "disk.img").Run()
	if err != nil {
		return "", fmt.Errorf("Could not execute qemu-img")
	}

	err = os.Chdir(currentDir)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", currentDir)
	}

	return deltaDiskDirectory + "/disk.img", nil
}

func (r *runner) CreateSeedImage(seedDirectory string) (string, error) {
	getisoimagePath, err := exec.LookPath("genisoimage")
	if err != nil {
		return "", fmt.Errorf("genisoimage is not installed on your PATH. Please, install it to run isolated container")
	}

	// Create user-data to be included in seed.img
	userDataString := `#cloud-config
runcmd:
 - mount -t 9p -o trans=virtio share_dir /mnt
 - chroot /mnt %s > /dev/hvc1 2>&1
 - init 0
`

	metaDataString := `#cloud-config
network-interfaces: |
  auto eth0
  iface eth0 inet static
  address %s
  netmask %s
  gateway %s
`

	var command string

	// TODO - there is no need for > 0 check
	if len(r.args) > 0 {
		args := []string{}
		for _, arg := range r.args {
			if strings.Contains(arg, " ") {
				args = append(args, fmt.Sprintf("'%s'", arg))
			} else {
				args = append(args, arg)
			}
		}
		argsAsString := strings.Join(args, " ")

		//command = fmt.Sprintf("%s %s", "/test", argsAsString)
		command = fmt.Sprintf("%s", argsAsString)
	} /*else {
		command = "/test"
		//command = command = lc.container.Path
	}*/
	//command = "ls"

	userData := []byte(fmt.Sprintf(userDataString, command))
	//metaData := []byte(fmt.Sprintf(metaDataString, lc.container.NetworkSettings.Networks["bridge"].IPAddress, netMask, lc.container.NetworkSettings.Networks["bridge"].Gateway))
	metaData := []byte(fmt.Sprintf(metaDataString, r.netInfo.IpAddr, r.netInfo.NetMask, r.netInfo.GateWay))

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not determine the current directory")
	}

	err = os.Chdir(seedDirectory)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", seedDirectory)
	}

	writeErrorUserData := ioutil.WriteFile("user-data", userData, 0700)
	if writeErrorUserData != nil {
		//return "", fmt.Errorf("Could not write user-data to /var/run/docker-qemu/%s", lc.container.ID)
		return "", fmt.Errorf("Could not write user-data to /var/run/docker-qemu/%s", "DDDDD")
	}

	writeErrorMetaData := ioutil.WriteFile("meta-data", metaData, 0700)
	if writeErrorMetaData != nil {
		//return "", fmt.Errorf("Could not write meta-data to /var/run/docker-qemu/%s", lc.container.ID)
		return "", fmt.Errorf("Could not write meta-data to /var/run/docker-qemu/%s", "DDDD")
	}

	logrus.Debugf("genisoimage path: %s", getisoimagePath)

	err = exec.Command(getisoimagePath, "-output", "seed.img", "-volid", "cidata", "-joliet", "-rock", "user-data", "meta-data").Run()
	if err != nil {
		return "", fmt.Errorf("Could not execute genisoimage")
	}

	err = os.Chdir(currentDir)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", currentDir)
	}

	return seedDirectory + "/seed.img", nil
}

type vmBaseConfig struct {
	numCPU           int
	DefaultMaxCpus   int
	DefaultMaxMem    int
	Memory           int
	OriginalDiskPath string
}

type vmMemory struct {
	Unit    string `xml:"unit,attr"`
	Content int    `xml:",chardata"`
}

type maxmem struct {
	Unit    string `xml:"unit,attr"`
	Slots   string `xml:"slots,attr"`
	Content int    `xml:",chardata"`
}

type vcpu struct {
	Placement string `xml:"placement,attr"`
	Current   string `xml:"current,attr"`
	Content   int    `xml:",chardata"`
}

type cell struct {
	Id     string `xml:"id,attr"`
	Cpus   string `xml:"cpus,attr"`
	Memory string `xml:"memory,attr"`
	Unit   string `xml:"unit,attr"`
}

type vmCpu struct {
	Mode string `xml:"mode,attr"`
}

type ostype struct {
	Arch    string `xml:"arch,attr"`
	Machine string `xml:"machine,attr"`
	Content string `xml:",chardata"`
}

type domainos struct {
	Supported string `xml:"supported,attr"`
	Type      ostype `xml:"type"`
}

type feature struct {
	Acpi acpi `xml:"acpi"`
}

type acpi struct {
}

type fspath struct {
	Dir string `xml:"dir,attr"`
}

type filesystem struct {
	Type       string `xml:"type,attr"`
	Accessmode string `xml:"accessmode,attr"`
	Source     fspath `xml:"source"`
	Target     fspath `xml:"target"`
}

type diskdriver struct {
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`
}

type disksource struct {
	File string `xml:"file,attr"`
}

type diskformat struct {
	Type string `xml:"type,attr"`
}

type backingstore struct {
	Type   string     `xml:"type,attr"`
	Index  string     `xml:"index,attr"`
	Format diskformat `xml:"format"`
	Source disksource `xml:"source"`
}

type disktarget struct {
	Dev string `xml:"dev,attr"`
	Bus string `xml:"bus,attr"`
}

type readonly struct {
}

type controller struct {
	Type  string `xml:"type,attr"`
	Model string `xml:"model,attr"`
}

type disk struct {
	Type         string        `xml:"type,attr"`
	Device       string        `xml:"device,attr"`
	Driver       diskdriver    `xml:"driver"`
	Source       disksource    `xml:"source"`
	BackingStore *backingstore `xml:"backingstore,omitempty"`
	Target       disktarget    `xml:"target"`
	Readonly     *readonly     `xml:"readonly,omitempty"`
}

type channsrc struct {
	Mode string `xml:"mode,attr"`
	Path string `xml:"path,attr"`
}

type constgt struct {
	Type string `xml:"type,attr,omitempty"`
	Port string `xml:"port,attr"`
}

type console struct {
	Type   string   `xml:"type,attr"`
	Source channsrc `xml:"source"`
	Target constgt  `xml:"target"`
}

type device struct {
	Emulator          string       `xml:"emulator"`
	Filesystems       []filesystem `xml:"filesystem"`
	Disks             []disk       `xml:"disk"`
	Consoles          []console    `xml:"console"`
	NetworkInterfaces []nic        `xml:"interface"`
	Controller        []controller `xml:"controller"`
}

type seclab struct {
	Type string `xml:"type,attr"`
}

type domain struct {
	XMLName    xml.Name  `xml:"domain"`
	Type       string    `xml:"type,attr"`
	Name       string    `xml:"name"`
	Memory     vmMemory  `xml:"memory"`
	MaxMem     *maxmem   `xml:"maxMemory,omitempty"`
	VCpu       vcpu      `xml:"vcpu"`
	OS         domainos  `xml:"os"`
	Features   []feature `xml:"features"`
	CPU        vmCpu     `xml:"cpu"`
	OnPowerOff string    `xml:"on_poweroff"`
	OnReboot   string    `xml:"on_reboot"`
	OnCrash    string    `xml:"on_crash"`
	Devices    device    `xml:"devices"`
	SecLabel   seclab    `xml:"seclabel"`
}

type nicmac struct {
	Address string `xml:"address,attr"`
}

type nicsrc struct {
	Bridge string `xml:"bridge,attr"`
}

type nicmodel struct {
	Type string `xml:"type,attr"`
}

type nic struct {
	Type   string   `xml:"type,attr"`
	Mac    nicmac   `xml:"mac"`
	Source nicsrc   `xml:"source"`
	Model  nicmodel `xml:"model"`
}

type netinfo struct {
	IpAddr  string
	MacAddr string
	NetMask string
	GateWay string
}

func (r *runner) DomainXml() (string, error) {
	baseCfg := &vmBaseConfig{
		numCPU:           1,
		DefaultMaxCpus:   2,
		DefaultMaxMem:    256,
		Memory:           256,
		OriginalDiskPath: "/var/lib/libvirt/images/disk.img.orig",
	}

	// Create directory for seed image and delta disk image
	directory := r.qemuDirectory

	deltaDiskImageLocation, err := r.CreateDeltaDiskImage(directory, baseCfg.OriginalDiskPath)
	if err != nil {
		return "", fmt.Errorf("Could not create delta disk image")
	}

	logrus.Debugf("Delta disk image location: %s", deltaDiskImageLocation)

	// Domain XML Formation
	dom := &domain{
		Type: "kvm",
		Name: r.container.ID(),
	}

	dom.Memory.Unit = "MiB"
	dom.Memory.Content = baseCfg.Memory

	dom.VCpu.Current = strconv.Itoa(baseCfg.numCPU)
	dom.VCpu.Content = baseCfg.numCPU

	dom.OS.Supported = "yes"
	dom.OS.Type.Content = "hvm"

	acpiFeature := feature{
		Acpi: acpi{},
	}
	dom.Features = append(dom.Features, acpiFeature)

	dom.SecLabel.Type = "none"

	dom.CPU.Mode = "host-model"

	dom.OnPowerOff = "destroy"
	dom.OnReboot = "destroy"
	dom.OnCrash = "destroy"

	diskimage := disk{
		Type:   "file",
		Device: "disk",
		Driver: diskdriver{
			Name: "qemu",
			Type: "qcow2",
		},
		Source: disksource{
			File: deltaDiskImageLocation,
		},
		BackingStore: &backingstore{
			Type:  "file",
			Index: "1",
			Format: diskformat{
				Type: "raw",
			},
			Source: disksource{
				File: baseCfg.OriginalDiskPath,
			},
		},
		Target: disktarget{
			Dev: "sda",
			Bus: "scsi",
		},
	}
	dom.Devices.Disks = append(dom.Devices.Disks, diskimage)

	seedimage := disk{
		Type:   "file",
		Device: "cdrom",
		Driver: diskdriver{
			Name: "qemu",
			Type: "raw",
		},
		Source: disksource{
			File: fmt.Sprintf("%s/seed.img", directory),
		},
		Target: disktarget{
			Dev: "sdb",
			Bus: "scsi",
		},
		Readonly: &readonly{},
	}
	dom.Devices.Disks = append(dom.Devices.Disks, seedimage)

	storageController := controller{
		Type:  "scsi",
		Model: "virtio-scsi",
	}
	dom.Devices.Controller = append(dom.Devices.Controller, storageController)

	//macAddress := lc.container.CommonContainer.NetworkSettings.Networks["bridge"].MacAddress
	// macAddress := "aa:bb:cc:dd:ee:ff"
	macAddress := r.netInfo.MacAddr
	networkInterface := nic{
		Type: "bridge",
		Mac: nicmac{
			Address: macAddress,
		},
		Source: nicsrc{
			Bridge: "docker0",
		},
		Model: nicmodel{
			Type: "virtio",
		},
	}
	dom.Devices.NetworkInterfaces = append(dom.Devices.NetworkInterfaces, networkInterface)

	fs := filesystem{
		Type:       "mount",
		Accessmode: "passthrough",
		Source: fspath{
			//Dir: lc.container.BaseFS,
			Dir: r.container.Config().Rootfs,
		},
		Target: fspath{
			Dir: "share_dir",
		},
	}
	dom.Devices.Filesystems = append(dom.Devices.Filesystems, fs)

	serialConsole := console{
		Type: "unix",
		Source: channsrc{
			Mode: "bind",
			Path: fmt.Sprintf("%s/serial.sock", directory),
		},
		Target: constgt{
			Type: "serial",
			Port: "0",
		},
	}
	dom.Devices.Consoles = append(dom.Devices.Consoles, serialConsole)
	//logrus.Debugf("Serial console socket location: %s", fmt.Sprintf("%s/serial.sock", lc.container.Config.QemuDirectory))
	logrus.Debugf("Serial console socket location: %s", fmt.Sprintf("%s/serial.sock", directory))
	vmConsole := console{
		Type: "unix",
		Source: channsrc{
			Mode: "bind",
			//Path: fmt.Sprintf("%s/arbritary.sock", lc.container.Config.QemuDirectory),
			Path: fmt.Sprintf("%s/arbritary.sock", directory),
		},
		Target: constgt{
			Type: "virtio",
			Port: "1",
		},
	}
	dom.Devices.Consoles = append(dom.Devices.Consoles, vmConsole)

	appConsole := console{
		Type: "unix",
		Source: channsrc{
			Mode: "bind",
			//Path: fmt.Sprintf("%s/app.sock", lc.container.Config.QemuDirectory),
			Path: fmt.Sprintf("%s/app.sock", directory),
		},
		Target: constgt{
			Type: "virtio",
			Port: "2",
		},
	}
	dom.Devices.Consoles = append(dom.Devices.Consoles, appConsole)
	//logrus.Debugf("Application console socket location: %s", fmt.Sprintf("%s/app.sock", lc.container.Config.QemuDirectory))
	logrus.Debugf("Application console socket location: %s", fmt.Sprintf("%s/app.sock", directory))
	data, err := xml.Marshal(dom)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func consoleReader(reader *bufio.Reader, output chan string) {
	line := []byte{}
	cr := false
	emit := false
	for {

		oneByte, err := reader.ReadByte()
		if err != nil {
			close(output)
			return
		}
		switch oneByte {
		case '\n':
			emit = !cr
			cr = false
		case '\r':
			emit = true
			cr = true
		default:
			cr = false
			line = append(line, oneByte)
		}
		if emit {
			output <- string(line)
			line = []byte{}
			emit = false
		}
	}
}

func (r *runner) run(config *specs.Process) (int, error) {
	qemuDirectory := fmt.Sprintf("/var/run/docker-qemu/%s", r.container.ID())
	err := os.MkdirAll(qemuDirectory, 0700)

	if err != nil {
		logrus.Error("Could not create directory /var/run/docker-qemu/%s : %s", r.container.ID(), err)
	}

	r.qemuDirectory = qemuDirectory
	r.args = config.Args
	logrus.Debugf("Testing debug from logrus")

	//CreateDeltaDiskImage(qemuDirectory, "/var/lib/libvirt/images/disk.img.orig")

	process, err := newProcess(*config)
	if err != nil {
		r.destroy()
		return -1, err
	}
	if len(r.listenFDs) > 0 {
		process.Env = append(process.Env, fmt.Sprintf("LISTEN_FDS=%d", len(r.listenFDs)), "LISTEN_PID=1")
		process.ExtraFiles = append(process.ExtraFiles, r.listenFDs...)
	}
	rootuid, err := r.container.Config().HostUID()
	if err != nil {
		r.destroy()
		return -1, err
	}
	rootgid, err := r.container.Config().HostGID()
	if err != nil {
		r.destroy()
		return -1, err
	}
	tty, err := setupIO(process, rootuid, rootgid, r.console, config.Terminal, r.detach || r.create)
	if err != nil {
		r.destroy()
		return -1, err
	}
	handler := newSignalHandler(tty, r.enableSubreaper)

	startFn := r.container.Start

	if !r.create {

		startFn = r.container.Run
	}
	//getStats, _ := r.container.Stats()
	//fmt.Println("domainXML7: %+v\n", r.container.Config().Hooks.Prestart[0])
	defer tty.Close()
	if err := startFn(process); err != nil {
		r.destroy()
		return -1, err
	}

	if err := tty.ClosePostStart(); err != nil {
		r.terminate(process)
		r.destroy()
		return -1, err
	}
	if r.pidFile != "" {
		if err := createPidFile(r.pidFile, process); err != nil {
			r.terminate(process)
			r.destroy()
			return -1, err
		}
	}
	pid, err := r.container.State()

	networkNamespacePath := pid.NamespacePaths[configs.NEWNET]
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))

	cmdName := dir + "/netinfo.sh"
	cmdArgs := []string{networkNamespacePath}
	cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		fmt.Println(os.Stderr, "There was an error running the command: ", err)

	}
	out := string(cmdOut)

	s := strings.Split(out, ",")

	r.netInfo.IpAddr, r.netInfo.MacAddr, r.netInfo.NetMask, r.netInfo.GateWay = s[0], s[1], s[2], s[3]
	fmt.Printf("COMMAND OUT MAC ADDRESS %s\n", r.netInfo.MacAddr)

	fmt.Printf("SPLIT OUT R NETINFO %s %s %s %s\n", r.netInfo.IpAddr, r.netInfo.MacAddr, r.netInfo.NetMask, r.netInfo.GateWay)
	r.CreateSeedImage(qemuDirectory)

	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		fmt.Errorf("Failed")
	}
	defer conn.Close()

	domainXml, err := r.DomainXml()
	if err != nil {
		logrus.Error("Fail to get domain xml configuration:", err)

	}
	logrus.Infof("domainXML: %v", domainXml)

	//var domain libvirt.VirDomain
	domain, err := conn.DomainDefineXML(domainXml)
	if err != nil {
		fmt.Println("FAILED DOMAIN FAILED")
		logrus.Error("Failed to launch domain ", err)

	}

	if domain == nil {
		fmt.Println("FAILED DOMAIN NIL")
		logrus.Error("Failed to launch domain as no domain in LibvirtContext")

	}

	err = domain.Create()
	if err != nil {
		logrus.Error("Fail to start qemu isolated container ", err)

	}

	logrus.Infof("Domain has started: %v", "AAAADDDD")
	logrus.Debugf("CONTAINER STRUCT")
	logrus.Debugf("%+v\n", r.container.Config().Networks[0].MacAddress)
	logrus.Debugf("CONTAINER STRUCT")

	appConsoleSockName := qemuDirectory + "/app.sock"

	var consoleConn net.Conn
	consoleConn, err = net.DialTimeout("unix", appConsoleSockName, time.Duration(10)*time.Second)

	if err != nil {
		logrus.Debugf("failed to connect  ", err.Error())
		//fmt.Fprint("failed to connect to ", appConsoleSockName, " ", err.Error(), "\n")

	}

	reader := bufio.NewReaderSize(consoleConn, 256)

	cout := make(chan string, 128)
	go consoleReader(reader, cout)

	for {
		line, ok := <-cout
		if ok {
			//fmt.Fprintln(stdout, line)
			fmt.Println(line)
		} else {
			break
		}
	}

	if r.detach || r.create {
		return 0, nil
	}
	status, err := handler.forward(process)
	if err != nil {
		r.terminate(process)
	}

	r.destroy()
	return status, err
}

func (r *runner) destroy() {
	if r.shouldDestroy {
		destroy(r.container)
	}
}

func (r *runner) terminate(p *libcontainer.Process) {
	p.Signal(syscall.SIGKILL)
	p.Wait()
}

func validateProcessSpec(spec *specs.Process) error {
	if spec.Cwd == "" {
		return fmt.Errorf("Cwd property must not be empty")
	}
	if !filepath.IsAbs(spec.Cwd) {
		return fmt.Errorf("Cwd must be an absolute path")
	}
	if len(spec.Args) == 0 {
		return fmt.Errorf("args must not be empty")
	}
	return nil
}

func startContainer(context *cli.Context, spec *specs.Spec, create bool) (int, error) {
	id := context.Args().First()
	if id == "" {
		return -1, errEmptyID
	}
	container, err := createContainer(context, id, spec)
	if err != nil {
		return -1, err
	}
	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}
	r := &runner{
		enableSubreaper: !context.Bool("no-subreaper"),
		shouldDestroy:   true,
		container:       container,
		listenFDs:       listenFDs,
		console:         context.String("console"),
		detach:          context.Bool("detach"),
		pidFile:         context.String("pid-file"),
		create:          create,
	}
	return r.run(&spec.Process)
}
