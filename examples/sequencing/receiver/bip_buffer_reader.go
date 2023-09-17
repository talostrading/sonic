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

	r.buf.Prefault()

	log.Println("running bip_buffer_reader")

	return r
}

func (r *BipBufferReader) Setup() {
	var (
		b      []byte
		onRead func(error, int, netip.AddrPort)
	)
	onRead = func(err error, nRead int, _ netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			seq, payloadSize, payload := r.decode(b)

			if r.first && seq != 1 {
				if seq != 1 {
					panic(fmt.Errorf(
						"sequencing must start at 1, not at %d", seq))
				}
				r.first = false
			}

			if *debug && *verbose {
				log.Printf(
					"read seq=%d n=%d payload=%s",
					seq,
					payloadSize,
					string(payload),
				)
			}

			start := time.Now()
			if seq == r.expected {
				r.expected++
				if *debug {
					log.Printf(
						"processed live seq=%d n_buffered=%d",
						seq,
						r.buf.Committed()/r.readbufsize,
					)
				}

				r.walkBuffer(seq)
			} else if seq > r.expected {
				b = r.buf.Commit(nRead)
				r.walkBuffer(seq)
			}
			diff := time.Since(start).Microseconds()

			if r.hist.TotalCount() < int64(*samples) {
				_ = r.hist.RecordValue(diff)
			} else {
				PrintHistogram(r.hist, r.buf.Committed()/r.readbufsize)
				r.hist.Reset()
			}

			if b = r.buf.Claim(r.readbufsize); b != nil {
				r.peer.AsyncRead(b, onRead)
			} else {
				panic("buffer full")
			}
		}
	}

	b = r.buf.Claim(r.readbufsize)
	r.peer.AsyncRead(b, onRead)
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

func (r *BipBufferReader) walkBuffer(seq int) {
	for {
		b := r.buf.Head()
		if b == nil {
			break
		}

		seq, _, _ := r.decode(b)
		if seq < r.expected {
			r.buf.Consume(r.readbufsize)
		} else if seq == r.expected {
			r.expected++
			r.buf.Consume(r.readbufsize)

			if *debug {
				log.Printf(
					"processed buff seq=%d n_buffered=%d",
					seq,
					r.buf.Committed()/r.readbufsize,
				)
			}
		} else {
			break
		}
	}
}
