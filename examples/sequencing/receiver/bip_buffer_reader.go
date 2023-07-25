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

	r.buf.Zero()

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
			if seq >= r.expected {
				// We only want to store expected or future sequence numbers.
				// Older ones are pointless to store as they will be Consumed
				// later on in process. However, consuming them also takes time,
				// so we're better off filtering them here.

				b = r.buf.Commit(nRead)

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
				r.process(seq, payloadSize, payload)
				diff := time.Since(start).Microseconds()

				if r.hist.TotalCount() < int64(*samples) {
					_ = r.hist.RecordValue(diff)
				} else {
					PrintHistogram(r.hist, r.buf.Committed()/r.readbufsize)
					r.hist.Reset()
				}
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

func (r *BipBufferReader) process(seq, n int, payload []byte) {
	if seq == r.expected {
		defer func() {
			// We defer the log in order to allow the consumption of the last
			// read packet in the loop below.
			if *debug {
				log.Printf(
					"processed live seq=%d n_buffered=%d",
					seq,
					r.buf.Committed()/r.readbufsize,
				)
			}
		}()

		r.expected++
	}

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
