package hypervisor

type VirtualMachine interface {
	ID() string
	Start() error
	Suspend() error
	Resume() error
	Stop() error
	Shutdown() error
	Kill() error
	Remove() error
}

type NetInfo struct {
	IpAddr  string
	MacAddr string
	NetMask string
	GateWay string
}

type VirtualMachineParams struct {
	Id      string
	NetInfo NetInfo
	Args    []string
	Path    string
	Rootfs  string
	DiskDir string
	NetworkNSPath string
}



type Hypervisor interface {
	GetConnection(url string) (conn interface{}, err error)
	CreateVM(vmParams VirtualMachineParams) (vm VirtualMachine, err error)
	GetVM(id string) (vm VirtualMachine, err error)
}


