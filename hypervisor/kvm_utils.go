package hypervisor

import (
	"os"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"bufio"
	"strconv"
	"encoding/xml"
	"path/filepath"
	"errors"
	"regexp"
)

func DeltaDiskImgPath(diskPath string) string{
	return diskPath + "/disk.img"
}

func SeedDiskImgPath(diskPath string) string{
	return diskPath + "/seed.img"
}

func (k *VirtualMachineParams) EnvPath(envVars []string) {
	for _, element := range envVars {
		envVar := strings.Split(element, "=")
		envVarName := envVar[0]
		envVarValue := envVar[1]

		envVarName = strings.TrimSpace(envVarName)
		envVarValue = strings.TrimSpace(envVarValue)

		if envVarName == "PATH" {
			k.Path = envVarValue
			break
		}
	}

}
func (k *VirtualMachineParams) CreateDeltaDiskImage() (string, error) {
	deltaImagePath, err := exec.LookPath("qemu-img")
	if err != nil {
		return "", fmt.Errorf("qemu-img is not installed on your PATH. Please, install it to run isolated qemu container")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not determine the current directory")
	}

	err = os.Chdir(k.DiskDir)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", k.DiskDir)
	}

	err = exec.Command(deltaImagePath, "create", "-f", "qcow2", "-b", OriginalDiskPath, "disk.img").Run()
	if err != nil {
		return "", fmt.Errorf("Could not execute qemu-img")
	}

	err = os.Chdir(currentDir)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", currentDir)
	}

	return DeltaDiskImgPath(k.DiskDir), nil
}

func (k *VirtualMachineParams) CreateSeedImage() (string, error) {
	getisoimagePath, err := exec.LookPath("genisoimage")
	if err != nil {
		return "", fmt.Errorf("genisoimage is not installed on your PATH. Please, install it to run isolated container")
	}

	// Create user-data to be included in seed.img
	userDataString := `#cloud-config
user: root
password: passw0rd
chpasswd: { expire: False }
ssh_pwauth: True
runcmd:
 - mount -t 9p -o trans=virtio share_dir /mnt
 - export PATH=%s
 - hostname %s
 MOUNT_PLACEHOLDER
 - chroot /mnt %s > /dev/hvc1 2>&1
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
	if len(k.Args) > 0 {
		args := []string{}
		for _, arg := range k.Args {
			if strings.Contains(arg, " ") {
				args = append(args, fmt.Sprintf("'%s'", arg))
			} else {
				args = append(args, arg)
			}
		}
		argsAsString := strings.Join(args, " ")
		command = fmt.Sprintf("%s", argsAsString)

	}

	userDataString = fmt.Sprintf(userDataString, k.Path, k.Id, command)

	r := regexp.MustCompile("MOUNT_PLACEHOLDER")
	m := regexp.MustCompile("/")

	if len(k.Mounts) == 0 {
		userDataString = r.ReplaceAllString(userDataString, "")
		n := regexp.MustCompile("\n\n")
		userDataString = n.ReplaceAllString(userDataString, "\n")
	} else {
		var mountString string
		var mountStringSlice []string
		for _, destination := range k.Mounts {
			mountStringSlice = append(mountStringSlice, " - mkdir -p /mnt" + destination)
			mountLabel := m.ReplaceAllString(destination, "_")
			mountStringSlice = append(mountStringSlice, " - mount "+ mountLabel+" /mnt"+destination+" -t 9p -o trans=virtio")
		}

		mountString = strings.Join(mountStringSlice, "\n")
		userDataString = r.ReplaceAllString(userDataString, mountString)
	}
	
	userData := []byte(userDataString)
	metaData := []byte(fmt.Sprintf(metaDataString, k.NetInfo.IpAddr, k.NetInfo.NetMask, k.NetInfo.GateWay))

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not determine the current directory")
	}

	err = os.Chdir(k.DiskDir)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", k.DiskDir)
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

	err = exec.Command(getisoimagePath, "-output", "seed.img", "-volid", "cidata", "-joliet", "-rock", "user-data", "meta-data").Run()
	if err != nil {
		return "", fmt.Errorf("Could not execute genisoimage")
	}

	err = os.Chdir(currentDir)
	if err != nil {
		return "", fmt.Errorf("Could not changed to directory %s", currentDir)
	}

	return SeedDiskImgPath(k.DiskDir), nil
}

func (k *VirtualMachineParams) NetworkInfo() error {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}

	cmdName := dir + "/netinfo.sh"
	cmdArgs := []string{k.NetworkNSPath}
	cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		return err

	}
	out := string(cmdOut)
	s := strings.Split(out, ",")
	if len(s) == 4 {
		k.NetInfo.IpAddr, k.NetInfo.MacAddr, k.NetInfo.NetMask, k.NetInfo.GateWay = s[0], s[1], s[2], s[3]
	} else {
		return errors.New("Parsing the output of netinfo script failed. "+ out)
	}

	return err
}

func (k *VirtualMachineParams) DomainXml() (string, error) {
	// TODO - read them from config.json
	baseCfg := &vmBaseConfig{
		numCPU:           NumCPU,
		DefaultMaxCpus:   DefaultMaxCpus,
		DefaultMaxMem:    DefaultMaxMem,
		Memory:           DefaultMem,
		OriginalDiskPath: OriginalDiskPath,
	}


	// Domain XML Formation
	dom := &domain{
		Type: "kvm",
		Name: k.Id,
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
			File: DeltaDiskImgPath(k.DiskDir),
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
			//File: fmt.Sprintf("%s/seed.img", directory),
			File: SeedDiskImgPath(k.DiskDir),
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

	macAddress := k.NetInfo.MacAddr
	if macAddress != "" {
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

	}

	dom.Devices.Graphics = graphics{Type:"vnc", Port:"-1"}
	fs := filesystem{
		Type:       "mount",
		Accessmode: "passthrough",
		Source: fspath{
			//Dir: r.container.Config().Rootfs,
			Dir : k.Rootfs,
		},
		Target: fspath{
			Dir: "share_dir",
		},
	}
	dom.Devices.Filesystems = append(dom.Devices.Filesystems, fs)

	for source, destination := range k.Mounts {
		m := regexp.MustCompile("/")
		mountLabel := m.ReplaceAllString(destination, "_")
		fs := filesystem{
			Type:       "mount",
			Accessmode: "passthrough",
			Source: fspath{
				//Dir: r.container.Config().Rootfs,
				Dir : source,
			},
			Target: fspath{
				Dir: mountLabel,
			},
		}
		dom.Devices.Filesystems = append(dom.Devices.Filesystems, fs)
	}

	serialConsole := console{
		Type: "unix",
		Source: channsrc{
			Mode: "bind",
			Path: fmt.Sprintf("%s/serial.sock", k.DiskDir),
		},
		Target: constgt{
			Type: "serial",
			Port: "0",
		},
	}
	dom.Devices.Consoles = append(dom.Devices.Consoles, serialConsole)
	vmConsole := console{
		Type: "unix",
		Source: channsrc{
			Mode: "bind",
			Path: fmt.Sprintf("%s/arbritary.sock", k.DiskDir),
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
			Path: fmt.Sprintf("%s/app.sock", k.DiskDir),
		},
		Target: constgt{
			Type: "virtio",
			Port: "2",
		},
	}
	dom.Devices.Consoles = append(dom.Devices.Consoles, appConsole)
	data, err := xml.Marshal(dom)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ConsoleReader(reader *bufio.Reader, output chan string) {
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

func createQemuDir(id string, err error) (string, error) {
	qemuDirectoryPath := fmt.Sprintf("/var/run/docker-qemu/%s", id)
	err = os.MkdirAll(qemuDirectoryPath, 0700)
	return qemuDirectoryPath, err
}


func NetworkInfo(err error, networkNamespacePath string) (error, []string) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	cmdName := dir + "/netinfo.sh"
	cmdArgs := []string{networkNamespacePath}
	cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		fmt.Println(os.Stderr, "There was an error running the command: ", err)

	}
	out := string(cmdOut)
	s := strings.Split(out, ",")
	return err, s
}