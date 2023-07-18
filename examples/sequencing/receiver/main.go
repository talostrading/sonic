package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
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
	if *debug {
		log.Println("----------------process----------------------")
	}

	defer func() {
		b.Consume(len(b.Data()))

		if *debug {
			log.Printf(
				"done processing buffer_len=%d",
				len(p.buffer),
			)
			log.Println("---------------------------------------------")
		}
	}()

	// Ignore anything we received before.
	if seq < p.expected {
		if *debug {
			log.Printf("ignoring seq=%d as we are on %d", seq, p.expected)
		}

		return
	}

	// Allow starting from sequence number other than 1.
	if p.expected == 1 {
		p.expected = seq
	}

	if seq == p.expected {
		if *debug {
			log.Printf("processing live seq=%d", seq)
		}

		p.expected++
		p.walkBuffer()
	} else {
		if *debug {
			log.Printf("buffering seq=%d", seq)
		}

		p.addToBuffer(seq, payload)
	}
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
			if *debug {
				log.Printf("processing buffered seq=%d", p.expected)
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
	if *debug {
		log.Println("----------------process----------------------")
	}

	defer func() {
		if *debug {
			log.Println("---------------------------------------------")
			log.Printf(
				"done processing sequencer_size=%d save_area_len=%d",
				p.sequencer.Size(),
				b.SaveLen(),
			)
		}
	}()

	// Ignore anything we received before.
	if seq < p.expected {
		if *debug {
			log.Printf("ignoring seq=%d as we are on %d", seq, p.expected)
		}
		b.Consume(len(b.Data()))
		return
	}

	// Allow starting from sequence number other than 1.
	if p.expected == 1 {
		p.expected = seq
	}

	if seq == p.expected {
		if *debug {
			log.Printf("processing live seq=%d", seq)
		}

		p.expected++
		b.Consume(len(b.Data()))
		p.walkBuffer(b)
	} else {
		if *debug {
			log.Printf("buffering seq=%d", seq)
		}

		p.addToBuffer(seq, payload, b)
	}
}

func (p *FastProcessor) walkBuffer(b *sonic.ByteBuffer) {
	for {
		slot, ok := p.sequencer.Pop(p.expected)
		if !ok {
			break
		}

		if *debug {
			log.Printf("processing buffered seq=%d", p.expected)
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

	sofar := 0
	hist := hdrhistogram.New(1, 10_000_000, 1)

	first := true

	var onRead func(error, int, netip.AddrPort)
	onRead = func(err error, n int, _ netip.AddrPort) {
		if err == nil {

			_ = b.ShrinkTo(n)
			b.Commit(n)

			if *debug {
				log.Printf("read %d bytes", n)
			}

			seq, payloadSize, payload := decode()
			if *debug {
				log.Printf(
					"received seq=%d n=%d payload=%s",
					seq,
					payloadSize,
					string(payload),
				)
			}

			if first {
				first = false
				log.Printf("received first packet seq=%d", seq)
			}

			start := time.Now()
			proc.Process(seq, payload, b)
			end := time.Now()

			if *samples > 0 {
				if sofar < *samples {
					_ = hist.RecordValue(end.Sub(start).Microseconds())
					start = end
					sofar++
				} else {
					log.Println("|||||||||||||||||||||||||||||||||||||||")

					log.Printf(
						"process latency min/avg/max/stddev = %d/%d/%d/%d",
						int(hist.Min()),
						int(hist.Mean()),
						int(hist.Max()),
						int(hist.StdDev()),
					)

					if proc.Type() == TypeFast {
						fp := proc.(*FastProcessor)
						log.Printf(
							"fast processor state save_len=%d read_len=%d write_len=%d sequencer_size=%d",
							b.SaveLen(),
							b.ReadLen(),
							b.WriteLen(),
							fp.sequencer.Size(),
						)
					}

					hist.Reset()
					sofar = 0

					log.Println("|||||||||||||||||||||||||||||||||||||||")
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
