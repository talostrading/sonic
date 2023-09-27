//go:build linux

package util

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func PinTo(cpuID int) error {
	cpuset := &unix.CPUSet{}
	cpuset.Set(cpuID)

	err := unix.SchedSetaffinity(0, cpuset)
	if err != nil {
		return err
	}

	verify := &unix.CPUSet{}
	err = unix.SchedGetaffinity(0, verify)
	if err != nil {
		return err
	}

	if verify.Count() != 1 {
		return fmt.Errorf("could not pin to CPU %d", cpuID)
	}

	return nil
}
