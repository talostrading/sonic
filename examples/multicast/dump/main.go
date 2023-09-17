package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/util"
)

var (
	lf             = flag.Bool("lf", false, "list network interfaces and quit")
	iname          = flag.String("iname", "", "interface to multicast with")
	addr           = flag.String("addr", "224.0.1.0:5001", "multicast group to join")
	ns             = flag.Int("ns", 128, "number of samples")
	checkDuplicate = flag.Bool("duplicate", false, "if true, check how many times we got a message")
)

func PrintIff(iff *net.Interface) string {
	return fmt.Sprintf(
		"interface name=%s mtu=%d index=%d addr=%s up=%v lo=%v p2p=%v multicast=%v",
		iff.Name,
		iff.MTU,
		iff.Index,
		iff.HardwareAddr,
		iff.Flags&net.FlagUp == net.FlagUp,
		iff.Flags&net.FlagLoopback == net.FlagLoopback,
		iff.Flags&net.FlagPointToPoint == net.FlagPointToPoint,
		iff.Flags&net.FlagMulticast == net.FlagMulticast,
	)
}

func main() {
	flag.Parse()

	if *lf {
		log.Println("listing interfaces")
		iffs, err := net.Interfaces()
		if err != nil {
			panic(err)
		}
		for _, iff := range iffs {
			log.Println(PrintIff(&iff))
		}

		return
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	log.Printf("getting interface %s\n", *iname)

	iff, err := net.InterfaceByName(*iname)
	if err != nil {
		panic(err)
	}

	log.Println(PrintIff(iff))

	log.Println("creating multicast client")

	multicastAddr, err := netip.ParseAddrPort(*addr)
	if err != nil {
		panic(err)
	}

	p, err := multicast.NewUDPPeer(ioc, "udp", multicastAddr.String())
	if err != nil {
		panic(err)
	}

	log.Printf("created multicast client, resolving address %s\n", *addr)

	udpAddr, err := net.ResolveUDPAddr("udp4", *addr)
	if err != nil {
		panic(err)
	}

	log.Printf("resolved address %s, joining multicast group\n", udpAddr)

	if err := p.JoinOn(multicast.IP(multicastAddr.Addr().String()), multicast.InterfaceName(*iname)); err != nil {
		panic(err)
	}

	log.Println("joined multicast, starting to read")

	received := make(map[string]int)
	if *checkDuplicate {
		t, err := sonic.NewTimer(ioc)
		if err != nil {
			panic(err)
		}
		t.ScheduleRepeating(5*time.Second, func() {
			fmt.Println("------------------------")
			for msg, count := range received {
				if count != 1 {
					fmt.Printf("received msg %s %d times\n", msg, count)
				}
			}
			fmt.Println("------------------------")
		})
	}

	b := make([]byte, iff.MTU)
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, from netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			b = b[:n]

			hexMsg := hex.EncodeToString(b)

			if *checkDuplicate {
				received[hexMsg]++
			}

			log.Printf("received from=%s n_bytes=%d payload=%s", from, n, hexMsg)

			b = b[:cap(b)]
			p.AsyncRead(b, onRead)
		}
	}
	p.AsyncRead(b, onRead)

	tracker := util.NewTrackerWithSamples(*ns)
	for {
		// I would use util.GetMonoTimeNanos() but cross-compiling is a mess,
		// because we link against the local glibc which might not be the glibc
		// on the remote.
		start := time.Now()
		n, _ := ioc.PollOne()
		if n > 0 {
			if stats := tracker.Record(time.Since(start).Nanoseconds()); stats != nil {
				log.Printf("loop latency (ns): %s\n", stats)
			}
		}
	}
}
