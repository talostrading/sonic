package sonic

import (
	"log"
	"testing"
)

func TestGetBoundDeviceNone(t *testing.T) {
	sock, err := NewSocket(SocketDomainIPv4, SocketTypeDatagram, SocketProtocolUDP)
	if err != nil {
		t.Fatal(err)
	}
	defer sock.Close()

	name, err := GetBoundDevice(sock.fd)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("socket is bound to %s", name)
}

// TODO No clue why this doesn't work
//func TestGetBoundDeviceSome(t *testing.T) {
//	iffs, err := net.Interfaces()
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	for _, iff := range iffs {
//		sock, err := NewSocket(SocketDomainIPv4, SocketTypeDatagram, SocketProtocolUDP)
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		log.Printf("binding socket fd=%d to %s", sock.fd, iff.Name)
//		if err := sock.BindToDevice(iff.Name); err != nil {
//			t.Fatal(err)
//		}
//
//		name, err := GetBoundDevice(sock.fd)
//		if err != nil {
//			t.Fatal(err)
//		}
//		log.Printf("socket is bound to %s", name)
//
//		_ = sock.Close()
//	}
//}
