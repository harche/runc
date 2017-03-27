package hypervisor

import (
	"testing"
)

func TestDeltaDiskImgPath(t *testing.T) {
	diskPath := DeltaDiskImgPath("/abc")
	if diskPath != "/abc/disk.img" {
		t.Error("Expected /abc/disk.img, got ", diskPath)
	}
}

func TestSeedDiskImgPath(t *testing.T) {
	diskPath := SeedDiskImgPath("/abc")
	if diskPath != "/abc/seed.img" {
		t.Error("Expected /abc/seed.img, got ", diskPath)
	}
}

func TestEnvPath(t *testing.T) {
	vmParmas := new(VirtualMachineParams)
	envVars := []string{"PATH=/abc", "GO=/mnt"}

	vmParmas.EnvPath(envVars)
	if vmParmas.Path != "/abc" {
		t.Error("Expected /abc, got ", vmParmas.Path)
	}
}
