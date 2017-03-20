package hypervisor

import (
	"github.com/libvirt/libvirt-go"
	"encoding/json"
	"os"
	"fmt"
//	"path/filepath"
	"path/filepath"
)


const (
	KVM = "KVM"
)

type VirtualMachine interface {
	ID() string
	Start() error
	Stop() error
	Shutdown() error
	Kill() error
	Remove() error
}

type KVMVirtualMachine struct {
	id string
}

func (k *KVMVirtualMachine) ID() string {
	return k.id
}

func (k *KVMVirtualMachine) Start() error {
	panic("implement me")
}

func (k *KVMVirtualMachine) Stop() error {
	panic("implement me")
}

func (k *KVMVirtualMachine) Shutdown() error {
	panic("implement me")
}

func (k *KVMVirtualMachine) Kill() error {
	panic("implement me")
}

func (k *KVMVirtualMachine) Remove() error {
	panic("implement me")
}

type Hypervisor interface {
	GetConnection(url string) (conn interface{}, err error)
	CreateVM(config interface{}) (vm VirtualMachine, err error)
	GetVM(id string) (vm VirtualMachine, err error)
}

type KVMHypervisor struct{
	conn *libvirt.Connect
}

func (k *KVMHypervisor) GetConnection(url string) (conn interface{}, err error) {
	fmt.Println("from get connection")
	k.conn, err = libvirt.NewConnect(url)
	if err != nil {
		fmt.Errorf("Failed")
	}
	return k.conn, nil
}

func (k *KVMHypervisor) CreateVM(config interface{}) (vm VirtualMachine, err error) {
	domainXml := config.(string)
	if k.conn == nil {
		hyperConn, _ := k.GetConnection("qemu:///system")
		k.conn = hyperConn.(*libvirt.Connect)
	}
	defer k.conn.Close()

	domain, err := k.conn.DomainDefineXML(domainXml)
	if err != nil {
		fmt.Println("FAILED DOMAIN FAILED %+v", err)

	}

	if domain == nil {
		fmt.Println("FAILED DOMAIN NIL")
	}

	err = domain.Create()
	if err != nil {
		fmt.Println("FAILED DOMAIN FAILED %+v", err)

	}

	return nil, nil
}

func (k *KVMHypervisor) GetVM(id string) (vm VirtualMachine, err error) {
	panic("implement me")
}

func HypFactory() (hypervisor Hypervisor, err error){
	config := ParseConfig()
	switch config.Name {
	case KVM:
		return new(KVMHypervisor), nil
	default:
		return new(KVMHypervisor), nil
	}
}

type Configuration struct {
	Name string
}

func ParseConfig() (config *Configuration) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	fmt.Println(dir)
	file, _ := os.Open(dir+"/hypervisor/config.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}
	return &configuration
}

