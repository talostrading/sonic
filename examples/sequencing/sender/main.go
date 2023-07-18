package main

import (
	"encoding/binary"
	"flag"
	"log"
	"math/rand"
	"net/netip"
	"runtime"
	dbg "runtime/debug"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
)

var (
	addr  = flag.String("addr", "224.0.0.224:8080", "multicast group address")
	iter  = flag.Int("iter", 10, "how many iterations, if 0, infinite")
	debug = flag.Bool("debug", false, "if true you can see what is sent")

	letters = []byte("abcdefghijklmnopqrstuvwxyz")
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	dbg.SetGCPercent(-1) // turn GC off

	flag.Parse()

	maddr, err := netip.ParseAddrPort(*addr)
	if err != nil {
		panic(err)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	p, err := multicast.NewUDPPeer(ioc, "udp", "")
	if err != nil {
		panic(err)
	}

	b := make([]byte, 256)
	var (
		current       uint32 = 1
		last          uint32 = 1
		backfill             = false
		backfillUntil uint32 = 1

		// 1 then increment random, backfill is true
		// if backfill is true, we need to backfill until we increment and then
		// is false
	)
	prepare := func() {
		binary.BigEndian.PutUint32(b, current) // sequence number

		// payload size
		n := rand.Intn(len(b) - 8 /* 4 for seq, 4 for payload size */)
		binary.BigEndian.PutUint32(b[4:], uint32(n))

		// payload
		for i := 0; i < n; i++ {
			b[8+i] = letters[i%len(letters)]
		}

		if *debug {
			log.Printf(
				"sending seq=%d n=%d payload=%s",
				current,
				n,
				string(b[8:8+n]),
			)
		}

		if backfill {
			last++
			if last == backfillUntil {
				backfill = false
				current = last + 1
			} else {
				current = last
			}
		} else {
			last = current
			increment := uint32(rand.Int31n(10) + 1)
			current += increment
			backfill = true
			backfillUntil = current
		}

	}

	var onWrite func(error, int)
	onWrite = func(err error, _ int) {
		if err == nil {
			prepare()
			p.AsyncWrite(b, maddr, onWrite)
		}
	}
	prepare()
	p.AsyncWrite(b, maddr, onWrite)

	if *iter == 0 {
		for {
			_, _ = ioc.PollOne()
		}
	} else {
		for i := 0; i < *iter; i++ {
			_, _ = ioc.PollOne()
		}
	}
}
