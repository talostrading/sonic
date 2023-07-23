package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net/netip"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
)

var _ Reader = &BipBufferReader{}

type BipBufferReader struct {
	bufsize     int
	readbufsize int
	peer        *multicast.UDPPeer

	buf          *sonic.BipBuffer
	expected     int
	first        bool
	hist         *hdrhistogram.Histogram
	lastBuffered int
}

func NewBipBufferReader(
	bufsize, readbufsize int,
	peer *multicast.UDPPeer,
) *BipBufferReader {
	r := &BipBufferReader{
		bufsize:     bufsize,
		readbufsize: readbufsize,
		peer:        peer,

		buf:      sonic.NewBipBuffer(bufsize),
		expected: 1,
		first:    false,
		hist:     hdrhistogram.New(1, 10_000_000, 1),
	}

	log.Println("running bip_buffer_reader")

	return r
}

func (r *BipBufferReader) Setup() {
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, _ netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			r.buf.Commit(n)
			entire := r.buf.Data()
			b := entire[len(entire)-r.readbufsize:]

			seq, n, payload := r.decode(b)

			if r.first {
				if seq != 1 {
					panic(fmt.Errorf(
						"first packet sequence number = %d != 1",
						seq,
					))
				}
				r.first = false
			}

			start := time.Now()
			r.process(seq, n, payload)
			diff := time.Since(start).Microseconds()

			if r.hist.TotalCount() < int64(*samples) {
				_ = r.hist.RecordValue(diff)
			} else {
				PrintHistogram(r.hist, r.buf.Committed()/r.readbufsize)
				r.hist.Reset()
			}

			if b := r.buf.Claim(r.readbufsize); b != nil {
				r.peer.AsyncRead(b, onRead)
			} else {
				panic("buffer full")
			}
		}
	}

	r.peer.AsyncRead(r.buf.Claim(r.readbufsize), onRead)
}

func (r *BipBufferReader) decode(b []byte) (seq, n int, payload []byte) {
	seq = int(binary.BigEndian.Uint32(b[:4]))
	n = int(binary.BigEndian.Uint32(b[4:]))
	if n > r.readbufsize {
		panic(fmt.Errorf(
			"something wrong as n = %d > %d b[hex]=%s",
			n,
			r.readbufsize,
			hex.EncodeToString(b)),
		)
	}
	payload = b[8 : 8+n]
	if len(payload) != n {
		panic("wrong payload")
	}
	return
}

func (r *BipBufferReader) process(seq, n int, payload []byte) {
	if seq == r.expected {
		if *debug {
			log.Printf("processing seq=%d", seq)
		}
		r.expected++
	}

	for {
		b := r.buf.Data()
		if b == nil {
			break
		}

		seq, _, _ := r.decode(b)
		if seq < r.expected {
			r.buf.Consume(r.readbufsize)
		} else if seq == r.expected {
			if *debug {
				log.Printf("processing buffered seq=%d", seq)
			}
			r.expected++
			r.buf.Consume(r.readbufsize)
		} else {
			break
		}
	}
	if *debug {
		log.Printf("committed=%d", r.buf.Committed())
	}
}
