package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net/netip"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/multicast"
	"github.com/csdenboer/sonic/util"
)

var _ Reader = &ByteBufferReader{}

type ByteBufferReader struct {
	proc        ByteBufferProcessor
	b           *sonic.ByteBuffer
	peer        *multicast.UDPPeer
	readBufSize int
	hist        *hdrhistogram.Histogram
	sofar       int
	first       bool
}

func NewByteBufferReader(
	proc ByteBufferProcessor,
	bufSize, readBufSize int,
	peer *multicast.UDPPeer,
) *ByteBufferReader {
	r := &ByteBufferReader{
		proc:        proc,
		peer:        peer,
		readBufSize: readBufSize,
		b:           sonic.NewByteBuffer(),
		hist:        hdrhistogram.New(1, 10_000_000, 1),
	}

	r.b.Reserve(bufSize)
	r.b.Prefault()
	log.Printf(
		"created sonic byte_buffer size=%s",
		util.ByteCountSI(int64(bufSize)),
	)

	return r
}

func (r *ByteBufferReader) Setup() {
	decode := func() (seq, n int, payload []byte) {
		seq = int(binary.BigEndian.Uint32(r.b.Data()[:4]))
		n = int(binary.BigEndian.Uint32(r.b.Data()[4:]))
		if n > *readBufferSize {
			panic(fmt.Errorf(
				"something wrong as n = %d > %d b.Data()[hex]=%s",
				n,
				*readBufferSize,
				hex.EncodeToString(r.b.Data())),
			)
		}
		r.b.Consume(8)
		payload = r.b.Data()[:n]
		return
	}

	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, _ netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			_ = r.b.ShrinkTo(n)
			r.b.Commit(n)

			seq, _, payload := decode()

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
			discarded := r.proc.Process(seq, payload, r.b)
			diff := time.Since(start).Microseconds()

			if r.sofar < *samples {
				if discarded > 0 {
					log.Printf(
						"discarded %d from buffer time=%d",
						discarded,
						diff,
					)
				}

				_ = r.hist.RecordValue(diff)
				r.sofar++
			} else {
				PrintHistogram(r.hist, r.proc.Buffered())
				r.hist.Reset()

				r.sofar = 0
			}

			if slice := r.b.ClaimFixed(r.readBufSize); slice != nil {
				r.peer.AsyncRead(slice, onRead)
			} else {
				panic("out of buffer space")
			}
		}
	}

	r.peer.AsyncRead(r.b.ClaimFixed(r.readBufSize), onRead)
}
