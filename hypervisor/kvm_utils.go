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
	"encoding/hex"
	"crypto/sha1"
	//"syscall"
	//"runtime"
)

func DeltaDiskImgPath(diskPath string) string{
	return diskPath + "/disk.img"
}

func SeedDiskImgPath(diskPath string) string{
	return diskPath + "/seed.img"
}

func (k *VirtualMachineParams) EnvPath(envVars []string) {
	envMap := make(map[string]string)
	for _, element := range envVars {


		envVar := strings.Split(element, "=")
		envVarName := envVar[0]
		envVarValue := envVar[1]

		envVarName = strings.TrimSpace(envVarName)
		envVarValue = strings.TrimSpace(envVarValue)
		envMap[envVarName] = envVarValue
		//if envVarName == "PATH" {
		//	k.Path = envVarValue
		//	break
		//}
	}
	k.Env = envMap

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
hostname: %s
runcmd:
 - mount -t 9p -o trans=virtio rootfs /mnt
 - sleep 2
 MOUNT_PLACEHOLDER
 - mkdir /cdrom
 - mount /dev/cdrom /cdrom
 - cp -p /cdrom/execute.sh /mnt/.
 - cp -p /cdrom/resolv.conf /mnt/etc/.
 - cp -p /cdrom/hosts /mnt/etc/.
 - cp -p /cdrom/systemd-data  /etc/systemd/system/myscript.service
 - mount --bind /dev/ /mnt/dev
 - mount --bind /proc /mnt/proc
 - systemctl enable myscript
 - service myscript start
`

	metaDataString := `#cloud-config
network-interfaces: |
  auto ens4
  iface ens4 inet static
  address %s
  netmask %s
  gateway %s
`

	systemdString := `[Unit]
Description=Sample Systemd
After=cloud-init.service

[Service]
Type=oneshot
ExecStart=/usr/sbin/chroot /mnt /execute.sh
ExecStop=/sbin/poweroff -f

[Install]
WantedBy=multi-user.target
`

	scriptContent := `#!/bin/sh
ENV_PLACEHOLDER
cd %s
%s > /dev/hvc1 2>&1
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

	userDataString = fmt.Sprintf(userDataString, k.Id)

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
		        //mountLabel=  Digest::MD5.new.update(opts[:hostpath]).to_s[0,31]
			h := sha1.New()
			h.Write([]byte(mountLabel))
			hashInBytes := h.Sum(nil)[:15	]
			mountLabel = hex.EncodeToString(hashInBytes)
			mountStringSlice = append(mountStringSlice, " - mount "+ mountLabel+" /mnt"+destination+" -t 9p -o trans=virtio")
		}

		mountString = strings.Join(mountStringSlice, "\n")
		userDataString = r.ReplaceAllString(userDataString, mountString)
	}

	systemdData := []byte(systemdString)
	scriptDataString := fmt.Sprintf(scriptContent, k.CwD, command)

	s := regexp.MustCompile("ENV_PLACEHOLDER")
	var envVars []string
	for envVar, envVal := range k.Env {
		envVars = append(envVars, "export " + envVar +"="+envVal)
	}
	envString := strings.Join(envVars, "\n")
	scriptDataString = s.ReplaceAllString(scriptDataString, envString)

	scriptData := []byte(scriptDataString)
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
		return "", fmt.Errorf("Could not write user-data for %s", k.Id)
	}

	writeErrorMetaData := ioutil.WriteFile("meta-data", metaData, 0700)
	if writeErrorMetaData != nil {
		return "", fmt.Errorf("Could not write meta-data for %s", k.Id)
	}

	writeErrorSystemdData := ioutil.WriteFile("systemd-data", systemdData, 0700)
	if writeErrorSystemdData != nil {
		return "", fmt.Errorf("Could not write systemd-data for %s", k.Id)
	}

	writeErrorScriptData := ioutil.WriteFile("execute.sh", scriptData, 0700)
	if writeErrorScriptData != nil {
		return "", fmt.Errorf("Could not write execute.sh for %s", k.Id)
	}

	writeErrorResolvConf := ioutil.WriteFile("resolv.conf", k.ResoveString, 0700)
	if writeErrorResolvConf != nil {
		return "", fmt.Errorf("Could not write resolv.conf for %s", k.Id)
	}

	writeErrorHosts := ioutil.WriteFile("hosts", k.HostsString, 0700)
	if writeErrorHosts != nil {
		return "", fmt.Errorf("Could not write hosts for %s", k.Id)
	}

	err = exec.Command(getisoimagePath, "-output", "seed.img", "-volid", "cidata", "-joliet", "-rock", "user-data", "meta-data", "systemd-data", "execute.sh", "resolv.conf", "hosts").Run()
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
	cmdArgs := []string{k.Pid}
	cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		return err

	}
	out := string(cmdOut)
	s := strings.Split(out, ",")
	if len(s) == 5 {
		k.NetInfo.IpAddr, k.NetInfo.MacAddr, k.NetInfo.NetMask, k.NetInfo.GateWay, k.NetInfo.Bridge = s[0], s[1], s[2], s[3], strings.TrimSpace(s[4])
	} else {
		return errors.New("Parsing the output of netinfo script failed. "+ out)
	}

	return err
}

func isDir(filePath string) (bool, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}

	mode := fi.Mode()
	if mode.IsDir() {
		return true, err
	}

	return false, nil

}




func (k *VirtualMachineParams) DomainXml() (string, error) {
	// TODO - read them from config.json

	config, err := ParseConfig()
	if err != nil {
		return "", err
	}

	numCpu := config.NumCPU
	if numCpu == 0 {
		numCpu = NumCPU
	}

	defaultMaxCpus := config.DefaultMaxCpus
	if defaultMaxCpus == 0 {
		defaultMaxCpus = DefaultMaxCpus
	}

	defaultMaxMem := config.DefaultMaxMem
	if defaultMaxMem == 0 {
		defaultMaxMem = DefaultMaxMem
	}

	defaultMem := config.DefaultMem
	if defaultMem == 0 {
		defaultMem = DefaultMem
	}

	originalDiskPath := config.OriginalDiskPath
	if originalDiskPath == "" {
		originalDiskPath = OriginalDiskPath
	}

	baseCfg := &vmBaseConfig{
		numCPU:           numCpu,
		DefaultMaxCpus:   defaultMaxCpus,
		DefaultMaxMem:    defaultMaxMem,
		Memory:           defaultMem,
		OriginalDiskPath: originalDiskPath,
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

	dom.CPU.Mode = "host-passthrough"

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
			Type: "direct",
			//Mac: nicmac{
			//	Address: macAddress,
			//},
			Source: sourceDev{
				Dev: k.NetInfo.Bridge,
				Mode: "passthrough",
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
		h := sha1.New()
		h.Write([]byte(mountLabel))
		hashInBytes := h.Sum(nil)[:15	]
		mountLabel = hex.EncodeToString(hashInBytes)

		isDestDir, err := isDir(source)
		if err != nil {
			return "", err
		}

		if !isDestDir{
			source, err = filepath.Abs(filepath.Dir(source))
			if err != nil {
				return "", err
			}

			if _, ok := k.Mounts[source]; ok {
				continue
			}
		}

		fs := filesystem{
			Type:       "mount",
			Accessmode: "passthrough",
			Source: fspath{
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
