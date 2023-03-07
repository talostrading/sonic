package main

import (
	"flag"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/util"
	"log"
	"net"
)

var (
	// for a server: iperf -c 224.0.1.0 -u -i 1
	addr = flag.String("addr", "224.0.1.0:5001", "multicast group to join")

	ns = flag.Int("ns", 128, "number of samples")
)

func main() {
	flag.Parse()

	ioc := sonic.MustIO()
	defer ioc.Close()

	iff, err := net.InterfaceByName("en0")
	if err != nil {
		panic(err)
	}
	log.Printf(
		"interface name=%s mtu=%d index=%d addr=%s\n",
		iff.Name, iff.MTU, iff.Index, iff.HardwareAddr)

	mc, err := sonic.NewUDPMulticastClient(ioc, iff, net.IPv4zero)
	if err != nil {
		panic(err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", *addr)
	if err != nil {
		panic(err)
	}

	if err := mc.Join(udpAddr); err != nil {
		panic(err)
	}

	b := make([]byte, iff.MTU)
	var onRead sonic.AsyncReadCallbackPacket
	onRead = func(err error, n int, addr net.Addr) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]
			log.Println(string(b))
			b = b[:cap(b)]
			mc.AsyncReadFrom(b, onRead)
		}
	}
	mc.AsyncReadFrom(b, onRead)

	tracker := util.NewTrackerWithSamples(*ns)
	for {
		start := util.GetMonoNanos()
		n, _ := ioc.PollOne()
		if n > 0 {
			end := util.GetMonoNanos()
			if stats := tracker.Record(end - start); stats != nil {
				log.Printf("loop latency (ns): %s\n", stats)
			}
		}
	}
}
