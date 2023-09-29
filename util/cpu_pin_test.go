package util

import (
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func getAllCPUs() ([]int, bool) {
	entries, err := os.ReadDir("/sys/devices/system/cpu")
	if err == nil {
		var cpus []int
		for _, entry := range entries {
			cpuid, ok := strings.CutPrefix(entry.Name(), "cpu")
			if ok {
				id, err := strconv.ParseInt(cpuid, 10, 64)
				if err == nil {
					cpus = append(cpus, int(id))
				}
			}
		}
		sort.Slice(cpus, func(i, j int) bool {
			return cpus[i] < cpus[j]
		})
		return cpus, true
	}
	return nil, false
}

func TestCPUPin(t *testing.T) {
	if err := PinTo(0); err != nil {
		t.Fatalf("could not pin to cpu 0 err=%v", err)
	}

	if cpus, ok := getAllCPUs(); ok {
		log.Printf("pinning to all cpus: %v", cpus)
		if err := PinTo(cpus...); err != nil {
			t.Fatalf("could not pin to cpus %v err=%v", cpus, err)
		}
	}
}
