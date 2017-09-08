package hypervisor

import (
	"path/filepath"
	"os"
	"encoding/json"

)

type Configuration struct {
	Name string
	OriginalDiskPath string
	NumCPU int
	DefaultMaxCpus int
	DefaultMaxMem int
	DefaultMem int
}


func ParseConfig() (config *Configuration, err error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, err
	}

	file, err := os.Open(dir+"/hypervisor/config.json")
	if err != nil {
		file, err = os.Open("/etc/runvm/config.json")
		if err != nil {
			return nil, err
		}
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		return nil, err
	}
	return &configuration, nil
}


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
	Bridge  string
}

type VirtualMachineParams struct {
	Id      string
	NetInfo NetInfo
        Detach  bool
	Args    []string
	Path    string
	Env     map[string]string
	Rootfs  string
	DiskDir string
	NetworkNSPath string
	Mounts  map[string]string
	ResoveString []byte
	HostsString []byte
	CwD	string
	Pid     string
}



type Hypervisor interface {
	GetConnection(url string) (conn interface{}, err error)
	CreateVM(vmParams VirtualMachineParams) (vm VirtualMachine, err error)
	GetVM(id string) (vm VirtualMachine, err error)
}


