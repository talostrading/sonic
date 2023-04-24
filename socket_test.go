package sonic

import (
	"log"
	"net"
	"testing"
)

func TestSocket_BindToDeviceIPv4(t *testing.T) {
	sock, err := NewSocket(SocketDomainIPv4, SocketTypeDatagram, SocketProtocolUDP)
	if err != nil {
		t.Fatal(err)
	}
	defer sock.Close()

	iffs, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}

	for _, iff := range iffs {
		log.Printf("attempting to bind socket to device=%s", iff.Name)
		if err := sock.BindToDevice(iff.Name); err != nil {
			t.Fatalf("could not bind socket to %s err=%v", iff.Name, err)
		} else if sock.BoundInterface() == nil {
			t.Fatal("should have set bound device")
		} else {
			log.Printf("bound socket fd=%d to device=%s", sock.RawFd(), iff.Name)
		}

		log.Printf("attempting to unbind socket from device=%s", iff.Name)
		if err := sock.UnbindFromDevice(); err != nil {
			t.Fatalf("could not unbind socket from %s err=%v", iff.Name, err)
		} else {
			log.Printf("unbound socket fd=%d from device=%s", sock.RawFd(), iff.Name)
		}
	}
}
