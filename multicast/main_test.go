package multicast

import (
	"fmt"
	"net"
	"os"
	"testing"
)

var testInterfaces []net.Interface

func TestMain(m *testing.M) {
	iffs, err := net.Interfaces()
	if err != nil {
		panic(fmt.Errorf("cannot get interfaces err=%v", err))
	}
	fmt.Println("found", len(iffs), "interfaces")
	for _, iff := range iffs {
		fmt.Printf(
			"interface name=%s index=%d mac=%s up=%v loopback=%v multicast=%v\n",
			iff.Name,
			iff.Index,
			iff.HardwareAddr,
			iff.Flags&net.FlagUp != 0,
			iff.Flags&net.FlagLoopback != 0,
			iff.Flags&net.FlagMulticast != 0,
		)
		if iff.Flags&net.FlagMulticast != 0 {
			testInterfaces = append(testInterfaces, iff)
		}
	}

	os.Exit(m.Run())
}
