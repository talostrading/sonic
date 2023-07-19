package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"runtime"
	"sort"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/util"

	_ "net/http/pprof"
)

var (
	addr     = flag.String("addr", "224.0.0.224:8080", "multicast group address")
	debug    = flag.Bool("debug", false, "if true, you can see what you receive")
	verbose  = flag.Bool("verbose", false, "if true, we also log ignored packets")
	slow     = flag.Bool("slow", true, "if true, use the slow processor, otherwise the fast one")
	samples  = flag.Int("iter", 4096, "number of samples to collect")
	justmax  = flag.Bool("justmax", true, "if true, we just get the max, otherwise min/avg/max/stddev")
	bufSize  = flag.Int("bufsize", 1024*256, "buffer size")
	maxSlots = flag.Int("maxslots", 1024, "max slots")
	busy     = flag.Bool("busy", true, "If true, busywait for events")
	iface    = flag.String("interface", "", "multicast interface")
	prof     = flag.Bool("prof", false, "If true, we profile the app")
)

type ProcessorType uint8

const (
	TypeSlow ProcessorType = iota
	TypeFast
)

type Processor interface {
	Process(seq int, payload []byte, b *sonic.ByteBuffer)
	Type() ProcessorType
	Buffered() int
}

type SlowProcessor struct {
	expected int
	buffer   []struct {
		seq int
		b   []byte
	}
}

func NewSlowProcessor() *SlowProcessor {
	p := &SlowProcessor{expected: 1}
	return p
}

func (p *SlowProcessor) Process(
	seq int,
	payload []byte,
	b *sonic.ByteBuffer,
) {
	defer b.Consume(len(b.Data()))

	// Ignore anything we received before.
	if seq < p.expected {
		if *debug && *verbose {
			log.Printf(
				"ignoring seq=%d(%d) as we are on %d n_bufferd=%d",
				seq,
				len(payload),
				p.expected,
				len(p.buffer),
			)
		}

		return
	}

	if seq == p.expected {
		if *debug {
			log.Printf(
				"processing live seq=%d(%d) n_buffered=%d",
				seq,
				len(payload),
				len(p.buffer),
			)
		}

		p.expected++
		p.walkBuffer()
	} else {
		buffered := p.addToBuffer(seq, payload)
		if buffered && *debug {
			log.Printf(
				"buffering seq=%d(%d) n_buffered=%d",
				seq,
				len(payload),
				len(p.buffer),
			)
		}
	}
}

func (p *SlowProcessor) addToBuffer(seq int, payload []byte) (buffered bool) {
	i := sort.Search(len(p.buffer), func(i int) bool {
		return p.buffer[i].seq >= seq
	})

	if i >= len(p.buffer) {
		b := make([]byte, len(payload))
		copy(b, payload)

		p.buffer = append(p.buffer, struct {
			seq int
			b   []byte
		}{
			seq,
			b,
		})

		buffered = true
	} else if i < len(p.buffer) && p.buffer[i].seq != seq {
		b := make([]byte, len(payload))
		copy(b, payload)

		p.buffer = append(p.buffer[:i+1], p.buffer[i:]...)
		p.buffer[i] = struct {
			seq int
			b   []byte
		}{
			seq,
			b,
		}
		buffered = true
	}

	return buffered
}

func (p *SlowProcessor) walkBuffer() {
	var newBuffer []struct {
		seq int
		b   []byte
	}
	for _, entry := range p.buffer {
		if entry.seq == p.expected {
			if *debug {
				log.Printf(
					"processing buffered seq=%d n_buffered=%d",
					p.expected,
					len(p.buffer),
				)
			}

			p.expected++
		} else {
			newBuffer = append(newBuffer, entry)
		}
	}
	p.buffer = newBuffer
}

func (p *SlowProcessor) Type() ProcessorType {
	return TypeSlow
}

func (p *SlowProcessor) Buffered() int {
	return len(p.buffer)
}

type FastProcessor struct {
	expected  int
	sequencer *sonic.SlotSequencer
}

func NewFastProcessor() *FastProcessor {
	p := &FastProcessor{
		expected:  1,
		sequencer: sonic.NewSlotSequencer(*maxSlots, *bufSize),
	}
	log.Printf(
		"created slot sequencer max_slots=%d buf_size=%d",
		*maxSlots,
		*bufSize,
	)
	return p
}

func (p *FastProcessor) Process(
	seq int,
	payload []byte,
	b *sonic.ByteBuffer,
) {
	// Ignore anything we received before.
	if seq < p.expected {
		if *debug && *verbose {
			log.Printf(
				"ignoring seq=%d(%d) as we are on %d n_buffered=%d",
				seq,
				len(payload),
				p.expected,
				p.sequencer.Size(),
			)
		}
		b.Consume(len(b.Data()))
		return
	}

	if seq == p.expected {
		if *debug {
			log.Printf(
				"processing live seq=%d(%d) n_buffered=%d n_bytes=%d",
				seq,
				len(payload),
				p.sequencer.Size(),
				p.sequencer.Bytes(),
			)
		}

		p.expected++
		p.walkBuffer(b)
		b.Consume(len(b.Data()))
	} else {
		buffered := p.addToBuffer(seq, payload, b)
		if *debug && buffered {
			log.Printf(
				"buffering seq=%d(%d) n_buffered=%d n_bytes=%d",
				seq,
				len(payload),
				p.sequencer.Size(),
				p.sequencer.Bytes(),
			)
		}
	}
}

func (p *FastProcessor) walkBuffer(b *sonic.ByteBuffer) {
	for {
		slot, ok := p.sequencer.Pop(p.expected)
		if !ok {
			break
		}

		if *debug {
			log.Printf(
				"processing buffered seq=%d n_buffered=%d n_bytes=%d",
				p.expected,
				p.sequencer.Size(),
				p.sequencer.Bytes(),
			)
		}

		p.expected++

		// handle p.expected
		_ = b.SavedSlot(slot)
		b.Discard(slot)
	}
}

func (p *FastProcessor) addToBuffer(
	seq int,
	payload []byte,
	b *sonic.ByteBuffer,
) bool {
	slot := b.Save(len(b.Data()))
	ok, err := p.sequencer.Push(seq, slot)
	if err != nil {
		panic(fmt.Errorf("could not push err=%s bytes=%d", err, p.sequencer.Bytes()))
	}
	if !ok {
		b.Discard(slot)
	}
	return ok
}

func (p *FastProcessor) Type() ProcessorType {
	return TypeFast
}

func (p *FastProcessor) Buffered() int {
	return p.sequencer.Size()
}

func main() {
	flag.Parse()

	if *prof {
		log.Println("profiling...")
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ioc := sonic.MustIO()
	defer ioc.Close()

	maddr, err := netip.ParseAddrPort(*addr)
	if err != nil {
		panic(err)
	}

	p, err := multicast.NewUDPPeer(ioc, "udp", maddr.String())
	if err != nil {
		panic(err)
	}

	multicastIP := maddr.Addr().String()
	if *iface == "" {
		log.Printf("joining %s", multicastIP)
		if err := p.Join(multicast.IP(multicastIP)); err != nil {
			panic(err)
		}
	} else {
		log.Printf("joining %s on %s", multicastIP, *iface)
		if err := p.JoinOn(
			multicast.IP(maddr.Addr().String()),
			multicast.InterfaceName(*iface),
		); err != nil {
			panic(err)
		}
	}

	var proc Processor
	if *slow {
		log.Printf("using slow")
		proc = NewSlowProcessor()
	} else {
		log.Printf("using fast")
		proc = NewFastProcessor()
	}

	b := sonic.NewByteBuffer()
	b.Reserve(*bufSize)
	b.Warm()

	log.Printf(
		"created sonic byte_buffer size=%s",
		util.ByteCountSI(int64(*bufSize)),
	)

	decode := func() (seq, n int, payload []byte) {
		seq = int(binary.BigEndian.Uint32(b.Data()[:4]))
		n = int(binary.BigEndian.Uint32(b.Data()[4:]))
		if n > 256 {
			panic(fmt.Errorf(
				"something wrong as n = %d > 256 b.Data()[hex]=%s",
				n,
				hex.EncodeToString(b.Data())),
			)
		}
		b.Consume(8)
		payload = b.Data()[:n]
		return
	}

	var (
		sofar = 0
		hist  *hdrhistogram.Histogram
		first       = true
		max   int64 = -10_000_000_000
	)

	if !*justmax {
		hist = hdrhistogram.New(1, 10_000_000, 1)
	}

	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, _ netip.AddrPort) {
		if err != nil {
			panic(err)
		} else {
			_ = b.ShrinkTo(n)
			b.Commit(n)

			seq, _, payload := decode()

			if first {
				if seq != 1 {
					panic(fmt.Errorf(
						"first packet sequence number = %d != 1",
						seq,
					))
				}
				first = false
			}

			start := time.Now()
			proc.Process(seq, payload, b)
			end := time.Now()

			if *samples > 0 {
				diff := end.Sub(start).Microseconds()
				start = end
				if sofar < *samples {
					if *justmax {
						if diff > max {
							max = diff
						}
					} else {
						_ = hist.RecordValue(diff)
					}
					sofar++
				} else {
					if *justmax {
						log.Printf(
							"process latency max = %dus n_buffered=%d",
							max,
							proc.Buffered(),
						)
					} else {
						log.Printf(
							"process latency min/avg/max/stddev = %d/%d/%d/%dus n_buffered=%d",
							int(hist.Min()),
							int(hist.Mean()),
							int(hist.Max()),
							int(hist.StdDev()),
							proc.Buffered(),
						)
						hist.Reset()
					}

					sofar = 0
					max = -10_000_000_000

				}
			}
			if slice := b.ClaimFixed(256); slice != nil {
				p.AsyncRead(slice, onRead)
			} else {
				panic("out of buffer space")
			}
		}
	}

	p.AsyncRead(b.ClaimFixed(256), onRead)

	log.Print("starting...")
	if *busy {
		log.Print("busy-waiting...")
		for {
			_, _ = ioc.PollOne()
		}
	} else {
		log.Print("yielding...")
		ioc.Run()
	}
}
