package hypervisor

import (
	"github.com/libvirt/libvirt-go"
	"fmt"
	"os"
	"encoding/xml"
	"net"
	"bufio"
	"time"
)


const (
	OriginalDiskPath  = "/var/lib/libvirt/images/disk.img.orig"
	NumCPU = 1
	DefaultMaxCpus = 2
	DefaultMaxMem = 256
	DefaultMem = 256
)

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
	Graphics          graphics     `xml:"graphics"`
}

type graphics struct {
	Type   string   `xml:"type,attr"`
	Port   string   `xml:"port,attr"`
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



type KVMVirtualMachine struct {
	id string
	domain *libvirt.Domain
}

func (k *KVMVirtualMachine) Suspend() error {
	err := k.domain.Suspend()
	if err != nil {
		fmt.Println("Fail to suspend qemu isolated container ", err)
		return err
	}
	return nil
}

func (k *KVMVirtualMachine) Resume() error {
	err := k.domain.Resume()
	if err != nil {
		fmt.Println("Fail to resume qemu isolated container ", err)
		return err
	}
	return nil
}

func (k *KVMVirtualMachine) ID() string {
	return k.id
}

func (k *KVMVirtualMachine) Start() error {
	panic("implement me")
}

func (k *KVMVirtualMachine) Stop() error {
	err := k.domain.Destroy()
	if err != nil {
		fmt.Println("Fail to stop qemu isolated container ", err)
		return err
	}
	return nil
}

func (k *KVMVirtualMachine) Shutdown() error {
	panic("implement me")
}

func (k *KVMVirtualMachine) Kill() error {
	err := k.Stop()
	if err == nil {
		err = k.Remove()
	}
	return err
}

func (k *KVMVirtualMachine) Remove() error {
	err := k.domain.Undefine()
	if err != nil {
		fmt.Println("Fail to remove qemu isolated container ", err)
		return err
	}

	if rerr := os.RemoveAll("/var/run/docker-qemu/" + k.id); err == nil {
		err = rerr
		return err
	}

	return nil
}




type KVMHypervisor struct{
	conn *libvirt.Connect
}

func (k *KVMHypervisor) GetConnection(url string) (conn interface{}, err error) {
	k.conn, err = libvirt.NewConnect(url)
	if err != nil {
		fmt.Errorf("Failed to get connection to qemu.")
	}
	return k.conn, nil
}

func (k *KVMHypervisor) CreateVM(vmParams VirtualMachineParams) (vm VirtualMachine, err error) {

	vmParams.DiskDir, err = createQemuDir(vmParams.Id, err)
	if err != nil {
		fmt.Errorf("Could not create directory /var/run/docker-qemu/%s : %s", vmParams.Id, err)
	}

	_ , err = vmParams.CreateDeltaDiskImage()
	if err != nil {
		fmt.Errorf("Could not create delta disk for vm %s : %s", vmParams.Id, err)
	}

	_ , err = vmParams.CreateSeedImage()
	if err != nil {
		fmt.Errorf("Could not create seed image for vm %s : %s", vmParams.Id, err)
	}

	domainXml, err := vmParams.DomainXml()
	if err != nil {
		fmt.Errorf("Could not create domain xml for vm %s : %s", vmParams.Id, err)
	}

	KVMConnection(k)
	defer k.conn.Close()
	
	domain, err := k.conn.DomainDefineXML(domainXml)
	if err != nil {
		fmt.Println("Could not define domain xml for vm %s : %+v",vmParams.Id, err)
		return nil, err


	}

	if domain == nil {
		fmt.Println("domain cannot be null for vm %s", vmParams.Id)
	}

	err = domain.Create()
	if err != nil {
		fmt.Println("Cannot create domain for vm %s : %+v", vmParams.Id, err)
		return nil, err

	}

	appConsoleSockName := vmParams.DiskDir + "/app.sock"

	var consoleConn net.Conn
	consoleConn, err = net.DialTimeout("unix", appConsoleSockName, time.Duration(10)*time.Second)

	if err != nil {
		fmt.Println("failed to get console conn %+v", err)

	}

	reader := bufio.NewReaderSize(consoleConn, 256)

	cout := make(chan string, 128)
        if !vmParams.Detach{
	    go ConsoleReader(reader, cout)

	    for {
		    line, ok := <-cout
		    if ok {
			    //fmt.Fprintln(stdout, line)
			    fmt.Println(line)
		    } else {
			    break
		    }
	    }
     }
	return nil, nil
}


func KVMConnection(k *KVMHypervisor) {
	if k.conn == nil {
		hyperConn, _ := k.GetConnection("qemu:///system")
		k.conn = hyperConn.(*libvirt.Connect)
	}
}

func (k *KVMHypervisor) GetVM(id string) (vm VirtualMachine, err error) {
	KVMConnection(k)
	defer k.conn.Close()

	kvmVirtualMachine := new(KVMVirtualMachine)
	domain, err := k.conn.LookupDomainByName(id)
	if err != nil {
		return nil, err
	}

	if domain == nil {
		return nil, err

	}

	kvmVirtualMachine.id = id
	kvmVirtualMachine.domain = domain

	return kvmVirtualMachine, nil
}

