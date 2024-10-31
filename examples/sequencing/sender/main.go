package main

import (
	"encoding/binary"
	"flag"
	"log"
	"math/rand"
	"net/netip"
	"runtime"
	dbg "runtime/debug"
	"time"

	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/multicast"
	"github.com/csdenboer/sonic/sonicerrors"
)

var (
	addr            = flag.String("addr", "224.0.0.224:8080", "multicast group address")
	iter            = flag.Int("iter", 10, "how many iterations, if 0, infinite")
	debug           = flag.Bool("debug", false, "if true you can see what is sent")
	rate            = flag.Duration("rate", 50*time.Microsecond, "sending rate")
	bindAddr        = flag.String("bind", "", "bind address")
	gap             = flag.Int("gap", 50, "maximum size of sequence number gaps")
	writeBufferSize = flag.Int("wbsize", 256, "write buffer size")
	duplicate       = flag.Int("duplicate", 2, "how many times to send the same packet")

	letters = []byte("abcdefghijklmnopqrstuvwxyz")
)

func main() {
	flag.Parse()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	dbg.SetGCPercent(-1) // turn GC off

	maddr, err := netip.ParseAddrPort(*addr)
	if err != nil {
		panic(err)
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	p, err := multicast.NewUDPPeer(ioc, "udp", *bindAddr)
	if err != nil {
		panic(err)
	}

	b := make([]byte, *writeBufferSize)
	var (
		seq          uint32 = 1
		continuation uint32 = 1
		until        uint32 = 1
		present             = true
	)

	jump := func() uint32 {
		return uint32(rand.Int31n(int32(*gap)) + 1)
	}
	until = jump()

	prepare := func() {
		binary.BigEndian.PutUint32(b, seq) // sequence number

		// payload size
		n := rand.Intn(len(b) - 8 /* 4 for seq, 4 for payload size */)
		binary.BigEndian.PutUint32(b[4:], uint32(n))

		// payload
		for i := 0; i < n; i++ {
			b[8+i] = letters[i%len(letters)]
		}

		if *debug {
			log.Printf(
				"sending seq=%d [present=%v until=%d] n=%d payload=%s",
				seq,
				present,
				until,
				n,
				string(b[8:8+n]),
			)
		}

		if present {
			if seq < until {
				seq++
			} else {
				continuation = seq + 1
				seq += jump()
				until = seq + jump()
				present = false
			}
		} else {
			if seq < until {
				seq++
			} else {
				seq = continuation
				present = true
				until += jump()
			}
		}
	}

	if *rate > 0 {
		log.Printf("sending at a rate of %s", *rate)

		t, err := sonic.NewTimer(ioc)
		if err != nil {
			panic(err)
		}
		err = t.ScheduleRepeating(*rate, func() {
			prepare()
			for i := 0; i < *duplicate; i++ {
				_, err := p.Write(b, maddr)
				for err == sonicerrors.ErrWouldBlock {
					_, err = p.Write(b, maddr)
				}
				if err != nil {
					panic(err)
				}
			}
		})
		if err != nil {
			panic(err)
		}
	} else {
		log.Print("sending as fast as possible")

		var onWrite func(error, int)
		onWrite = func(err error, _ int) {
			if err == nil {
				prepare()
				// Send the same packet 10 times to increase the likelihood of
				// arrival.
				for i := 0; i < 10; i++ {
					p.AsyncWrite(b, maddr, onWrite)
				}
			} else {
				panic(err)
			}
		}

		prepare()
		// Send the same packet 10 times to increase the likelihood of arrival.
		for i := 0; i < 10; i++ {
			p.AsyncWrite(b, maddr, onWrite)
		}
	}

	log.Print("starting...")
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
