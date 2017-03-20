package hypervisor

import (
	"github.com/libvirt/libvirt-go"
	"fmt"
)


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

type KVMHypervisor struct{
	conn *libvirt.Connect
}

func (k *KVMHypervisor) GetConnection(url string) (conn interface{}, err error) {
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
