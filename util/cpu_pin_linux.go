//go:build linux

package util

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func PinTo(cpus ...int) error {
	set := &unix.CPUSet{}
	for _, cpu := range cpus {
		set.Set(cpu)
	}

	err := unix.SchedSetaffinity(0, set)
	if err != nil {
		return err
	}

	verify := &unix.CPUSet{}
	err = unix.SchedGetaffinity(0, verify)
	if err != nil {
		return err
	}

	if verify.Count() != len(cpus) {
		return fmt.Errorf("could not pin to CPUs %v", cpus)
	}
	for _, cpu := range cpus {
		if !verify.IsSet(cpu) {
			return fmt.Errorf("could not pin to CPUs %v", cpus)
		}
	}

	return nil
}
