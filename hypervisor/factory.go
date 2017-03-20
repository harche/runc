package hypervisor

import (
	"encoding/json"
	"os"
	"fmt"
	"path/filepath"
)


const (
	KVM = "KVM"
)

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
	file, _ := os.Open(dir+"/hypervisor/config.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}
	return &configuration
}

