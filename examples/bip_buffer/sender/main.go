package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/netip"
	"reflect"
	"time"

	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/sonicerrors"
)

var (
	rate        = flag.Duration("rate", time.Second, "rate at which to send data")
	multicastIP = flag.String("multicastIP", "224.0.0.224", "multicast address on which to send")
	debug       = flag.Bool("debug", false, "if true, print what is sent")
	maxsize     = flag.Int("maxsize", 64, "max packet size")
	ifaceName   = flag.String("interface", "", "interface on which to send on")
	port        = flag.Int("port", 8080, "port on which to send")

	letters = []byte("abcdefghijklmnopqrstuvwxyz")
)

func main() {
	flag.Parse()

	if *port == 0 {
		panic("port must not be 0")
	}

	if *maxsize < 1<<3 || *maxsize > 1<<16 {
		panic(fmt.Errorf("maxsize must be in [2**3, 2**16]"))
	}

	ioc := sonic.MustIO()
	defer ioc.Close()

	bindAddr, err := bind(*ifaceName, *port)
	if err != nil {
		panic(err)
	}

	peer, err := multicast.NewUDPPeer(ioc, "udp", bindAddr)
	if err != nil {
		panic(err)
	}

	multicastAddr, err := netip.ParseAddrPort(
		fmt.Sprintf("%s:%d", *multicastIP, *port),
	)
	if err != nil {
		panic(err)
	}
	log.Printf("sending on multicast_addr=%s", multicastAddr)

	var (
		b          = make([]byte, *maxsize)
		seq uint32 = 1
	)
	prepare := func() {
		b = b[:cap(b)]

		binary.BigEndian.PutUint32(b, seq)
		n := rand.Int31n(int32(*maxsize - 8))
		binary.BigEndian.PutUint32(b[4:], uint32(n))

		payload := b[8:]
		for i := 0; i < int(n); i++ {
			payload[i] = letters[i%len(letters)]
		}

		b = b[:8+n]

		if *debug {
			log.Printf(
				"sending seq=%d n=%d payload=%s",
				seq,
				n,
				string(b[8:]),
			)
		}

		seq++
	}

	ticker, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}
	err = ticker.ScheduleRepeating(*rate, func() {
		prepare()
		_, err = peer.Write(b, multicastAddr)
		for errors.Is(err, sonicerrors.ErrWouldBlock) ||
			errors.Is(err, sonicerrors.ErrNoBufferSpaceAvailable) {
			_, err = peer.Write(b, multicastAddr)
		}
		if err != nil {
			panic(reflect.TypeOf(err))
		}
	})
	if err != nil {
		panic(err)
	}
	log.Printf("sending every %s", *rate)

	for {
		_, _ = ioc.PollOne()
	}
}

func bind(interfaceName string, port int) (string, error) {
	if interfaceName == "" {
		return "", nil
	}

	var bindIP string

	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return "", err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		var toParse string
		switch addr := addr.(type) {
		case *net.IPNet:
			toParse = addr.IP.String()
		case *net.IPAddr:
			toParse = addr.String()
		default:
			return "", fmt.Errorf("unknown addr type=%+v", addr)
		}
		parsed, err := netip.ParseAddr(toParse)
		if err != nil {
			return "", err
		}
		if parsed.Is4() || parsed.Is4In6() {
			bindIP = parsed.String()
			break
		}
	}
	if bindIP == "" {
		return "", fmt.Errorf("cannot send on interface=%s", interfaceName)
	} else {
		if port == 0 {
			return "", fmt.Errorf("port must not be 0")
		}
		log.Printf(
			"sending on interface=%s addr=%s:%d",
			interfaceName,
			bindIP,
			port,
		)
	}

	return fmt.Sprintf("%s:%d", bindIP, port), nil
}
