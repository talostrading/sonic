package main

import (
	"encoding/binary"
	"flag"
	"log"
	"net/netip"
	"runtime"
	dbg "runtime/debug"
	"sort"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/talostrading/sonic"
	"github.com/talostrading/sonic/multicast"
	"github.com/talostrading/sonic/util"
)

var (
	addr     = flag.String("addr", "224.0.0.224:8080", "multicast group address")
	debug    = flag.Bool("debug", false, "if true, you can see what you receive")
	slow     = flag.Bool("slow", true, "if true, use the slow processor, otherwise the fast one")
	samples  = flag.Int("iter", 4096, "number of samples to collect")
	bufSize  = flag.Int("bufsize", 1024*1024*256, "buffer size")
	maxSlots = flag.Int("maxslots", 1024, "max slots")
	busy     = flag.Bool("busy", true, "If true, busywait for events")
)

type ProcessorType uint8

const (
	TypeSlow ProcessorType = iota
	TypeFast
)

type Processor interface {
	Process(seq int, payload []byte, b *sonic.ByteBuffer)
	Type() ProcessorType
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
	if seq == p.expected {
		p.expected++
		p.walkBuffer()
	} else {
		p.addToBuffer(seq, payload)
	}
	b.Consume(len(b.Data()))
}

func (p *SlowProcessor) addToBuffer(seq int, payload []byte) {
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
	}
}

func (p *SlowProcessor) walkBuffer() {
	var newBuffer []struct {
		seq int
		b   []byte
	}
	for _, entry := range p.buffer {
		if entry.seq == p.expected {
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
	if p.expected == 1 {
		p.expected = seq
	}

	if seq == p.expected {
		p.expected++
		b.Consume(len(b.Data()))
		p.walkBuffer(b)
	} else {
		p.addToBuffer(seq, payload, b)
	}
}

func (p *FastProcessor) walkBuffer(b *sonic.ByteBuffer) {
	for {
		slot, ok := p.sequencer.Pop(p.expected)
		if !ok {
			break
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
) {
	p.sequencer.Push(seq, b.Save(len(b.Data())))
}

func (p *FastProcessor) Type() ProcessorType {
	return TypeFast
}

func (p *FastProcessor) Dump() {
	log.Printf(
		"sequencer bytes=%d size=%d",
		p.sequencer.Bytes(),
		p.sequencer.Size(),
	)
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	dbg.SetGCPercent(-1) // turn GC off

	flag.Parse()

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

	if err := p.Join(multicast.IP(maddr.Addr().String())); err != nil {
		panic(err)
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
		b.Consume(8)
		payload = b.Data()[:n]
		return
	}

	sofar := 0
	hist := hdrhistogram.New(1, 10_000_000, 1)

	start := time.Now()
	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, _ netip.AddrPort) {
		if err == nil {

			_ = b.ShrinkTo(n)
			b.Commit(n)

			seq, payloadSize, payload := decode()
			if *debug {
				log.Printf(
					"received seq=%d n=%d payload=%s",
					seq,
					payloadSize,
					string(payload),
				)
			}

			proc.Process(seq, payload, b)

			if *samples > 0 {
				if sofar < *samples {
					end := time.Now()
					_ = hist.RecordValue(end.Sub(start).Microseconds())
					start = end
					sofar++
				} else {
					log.Printf(
						"report min/avg/max/stddev = %d/%d/%d/%d",
						int(hist.Min()),
						int(hist.Mean()),
						int(hist.Max()),
						int(hist.StdDev()),
					)
					hist.Reset()
					start = time.Now()
					sofar = 0
				}
			}
			if slice := b.ClaimFixed(256); slice != nil {
				p.AsyncRead(slice, onRead)
			} else {
				if proc.Type() == TypeFast {
					proc.(*FastProcessor).Dump()
				}
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
