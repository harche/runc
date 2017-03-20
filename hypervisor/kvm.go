package hypervisor

import (
	"github.com/libvirt/libvirt-go"
	"fmt"
	"os"
)


type KVMVirtualMachine struct {
	id string
	domain *libvirt.Domain
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
	err := k.domain.Undefine()
	if err != nil {
		fmt.Println("Fail to start qemu isolated container ", err)
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
		fmt.Errorf("Failed")
	}
	return k.conn, nil
}

func (k *KVMHypervisor) CreateVM(config interface{}) (vm VirtualMachine, err error) {
	domainXml := config.(string)
	KVMConnection(k)
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
		fmt.Println("Failed to launch domain ", err)

	}

	if domain == nil {
		fmt.Println("Failed to launch domain as no domain in LibvirtContext")
		return nil, err

	}

	kvmVirtualMachine.id = id
	kvmVirtualMachine.domain = domain

	return kvmVirtualMachine, nil
}

