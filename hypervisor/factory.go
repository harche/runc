package hypervisor


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

