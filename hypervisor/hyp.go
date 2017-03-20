package hypervisor

type VirtualMachine interface {
	ID() string
	Start() error
	Stop() error
	Shutdown() error
	Kill() error
	Remove() error
}

type Hypervisor interface {
	GetConnection(url string) (conn interface{}, err error)
	CreateVM(config interface{}) (vm VirtualMachine, err error)
	GetVM(id string) (vm VirtualMachine, err error)
}


