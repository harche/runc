package hypervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
)


const (
	KVM = "KVM"
)

func HypFactory() (hypervisor Hypervisor, err error){
	config, err := ParseConfig()
	if err != nil {
		return nil, err
	}
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

func ParseConfig() (config *Configuration, err error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, err
	}

	file, err := os.Open(dir+"/hypervisor/config.json")
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		return nil, err
	}
	return &configuration, nil
}

