package main

import (
	"flag"
	"log"
	"net/http"
	"net/netip"
	"runtime"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/util"

	_ "net/http/pprof"
)

var (
	which          = flag.String("which", "simple", "one of: simple,pool,fenwick")
	addr           = flag.String("addr", "224.0.0.224:8080", "multicast group address")
	debug          = flag.Bool("debug", false, "if true, you can see what you receive")
	verbose        = flag.Bool("verbose", false, "if true, we also log ignored packets")
	samples        = flag.Int("iter", 4096, "number of samples to collect")
	bufSize        = flag.Int("bufsize", 1024*256, "buffer size")
	maxSlots       = flag.Int("maxslots", 1024, "max slots")
	iface          = flag.String("interface", "", "multicast interface")
	prof           = flag.Bool("prof", false, "If true, we profile the app")
	readBufferSize = flag.Int("rbsize", 256, "read buffer size")
	pinCPU         = flag.Int("pincpu", -1, "which cpu to pin to")
)

func main() {
	flag.Parse()

	if *prof {
		log.Println("profiling...")
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	if *pinCPU >= 0 {
		log.Printf("pinning process to cpu=%d", *pinCPU)
		if err := util.PinTo(*pinCPU); err != nil {
			panic(err)
		}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ioc := sonic.MustIO()
	defer ioc.Close()

	maddr, err := netip.ParseAddrPort(*addr)
	if err != nil {
		panic(err)
	}

	peer, err := multicast.NewUDPPeer(ioc, "udp", maddr.String())
	if err != nil {
		panic(err)
	}

	multicastIP := maddr.Addr().String()
	if *iface == "" {
		log.Printf("joining %s", multicastIP)
		if err := peer.Join(multicast.IP(multicastIP)); err != nil {
			panic(err)
		}
	} else {
		log.Printf("joining %s on %s", multicastIP, *iface)
		if err := peer.JoinOn(
			multicast.IP(maddr.Addr().String()),
			multicast.InterfaceName(*iface),
		); err != nil {
			panic(err)
		}
	}

	var reader Reader
	switch *which {
	case "simple":
		reader = NewByteBufferReader(
			NewSimpleProcessor(),
			*bufSize,
			*readBufferSize,
			peer,
		)
	case "pool":
		reader = NewByteBufferReader(
			NewPoolProcessor(),
			*bufSize,
			*readBufferSize,
			peer,
		)
	case "fenwick":
		reader = NewByteBufferReader(
			NewFenwickProcessor(),
			*bufSize,
			*readBufferSize,
			peer,
		)
	case "bip":
		reader = NewBipBufferReader(*bufSize, *readBufferSize, peer)
	default:
		panic("unknown processor type")
	}

	reader.Setup()

	for {
		_, _ = ioc.PollOne()
	}
}
