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
	"github.com/valyala/bytebufferpool"

	_ "net/http/pprof"
)

var (
	which          = flag.String("which", "alloc", "one of: alloc,pool,noalloc")
	addr           = flag.String("addr", "224.0.0.224:8080", "multicast group address")
	debug          = flag.Bool("debug", false, "if true, you can see what you receive")
	verbose        = flag.Bool("verbose", false, "if true, we also log ignored packets")
	samples        = flag.Int("iter", 4096, "number of samples to collect")
	bufSize        = flag.Int("bufsize", 1024*256, "buffer size")
	maxSlots       = flag.Int("maxslots", 1024, "max slots")
	iface          = flag.String("interface", "", "multicast interface")
	prof           = flag.Bool("prof", false, "If true, we profile the app")
	readBufferSize = flag.Int("rbsize", 256, "read buffer size")
	allocateSome   = flag.Bool("allocate", false, "if true make some pointless inline allocations to put more pressure on GC")
)

type Processor interface {
	Process(seq int, payload []byte, b *sonic.ByteBuffer)
	Buffered() int
}

type AllocProcessor struct {
	expected int
	buffer   []struct {
		seq int
		b   []byte
	}
}

func NewAllocProcessor() *AllocProcessor {
	p := &AllocProcessor{expected: 1}
	log.Println("using plain alloc processor")
	return p
}

func (p *AllocProcessor) Process(
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

func (p *AllocProcessor) addToBuffer(seq int, payload []byte) (buffered bool) {
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

func (p *AllocProcessor) walkBuffer() {
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

func (p *AllocProcessor) Buffered() int {
	return len(p.buffer)
}

type PoolAllocProcessor struct {
	expected int
	buffer   []struct {
		seq int
		b   *bytebufferpool.ByteBuffer
	}
}

func NewPoolAllocProcessor() *PoolAllocProcessor {
	p := &PoolAllocProcessor{expected: 1}
	log.Println("using pool alloc processor")
	return p
}

func (p *PoolAllocProcessor) Process(
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

func (p *PoolAllocProcessor) addToBuffer(seq int, payload []byte) (buffered bool) {
	i := sort.Search(len(p.buffer), func(i int) bool {
		return p.buffer[i].seq >= seq
	})

	if i >= len(p.buffer) {
		bb := bytebufferpool.Get()
		_, _ = bb.Write(payload)

		p.buffer = append(p.buffer, struct {
			seq int
			b   *bytebufferpool.ByteBuffer
		}{
			seq,
			bb,
		})

		buffered = true
	} else if i < len(p.buffer) && p.buffer[i].seq != seq {
		bb := bytebufferpool.Get()
		_, _ = bb.Write(payload)

		p.buffer = append(p.buffer[:i+1], p.buffer[i:]...)
		p.buffer[i] = struct {
			seq int
			b   *bytebufferpool.ByteBuffer
		}{
			seq,
			bb,
		}
		buffered = true
	}

	return buffered
}

func (p *PoolAllocProcessor) walkBuffer() {
	var newBuffer []struct {
		seq int
		b   *bytebufferpool.ByteBuffer
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

			bytebufferpool.Put(entry.b)
		} else {
			newBuffer = append(newBuffer, entry)
		}
	}
	p.buffer = newBuffer
}

func (p *PoolAllocProcessor) Buffered() int {
	return len(p.buffer)
}

type NoAllocProcessor struct {
	expected  int
	sequencer *sonic.SlotSequencer
}

func NewNoAllocProcessor() *NoAllocProcessor {
	p := &NoAllocProcessor{
		expected:  1,
		sequencer: sonic.NewSlotSequencer(*maxSlots, *bufSize),
	}
	log.Printf(
		"created slot sequencer max_slots=%d buf_size=%d",
		*maxSlots,
		*bufSize,
	)
	log.Println("using no alloc processor")
	return p
}

func (p *NoAllocProcessor) Process(
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

func (p *NoAllocProcessor) walkBuffer(b *sonic.ByteBuffer) {
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

func (p *NoAllocProcessor) addToBuffer(
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

func (p *NoAllocProcessor) Buffered() int {
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
	switch *which {
	case "alloc":
		proc = NewAllocProcessor()
	case "pool":
		proc = NewPoolAllocProcessor()
	case "noalloc":
		proc = NewNoAllocProcessor()
	default:
		panic("unknown processor type")
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
		if n > *readBufferSize {
			panic(fmt.Errorf(
				"something wrong as n = %d > %d b.Data()[hex]=%s",
				n,
				*readBufferSize,
				hex.EncodeToString(b.Data())),
			)
		}
		b.Consume(8)
		payload = b.Data()[:n]
		return
	}

	var (
		sofar = 0
		hist  = hdrhistogram.New(1, 10_000_000, 1)
		first = true
	)

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

			proc.Process(seq, payload, b)

			if *allocateSome {
				sum := 0
				for i := 0; i < 10; i++ {
					b := make([]byte, 4096)
					for j := 0; j < len(b); j++ {
						b[j] = byte(j % 255)
						sum += int(b[j])
					}
				}

				if *verbose {
					// just to not have it optimized away
					log.Printf("pointless sum=%d", sum)
				}
			}

			if slice := b.ClaimFixed(*readBufferSize); slice != nil {
				p.AsyncRead(slice, onRead)
			} else {
				panic("out of buffer space")
			}
		}
	}

	p.AsyncRead(b.ClaimFixed(*readBufferSize), onRead)

	log.Print("starting...")
	if *samples > 0 {
		log.Println("measuring loop latency")

		for {
			start := time.Now()
			n, _ := ioc.PollOne()
			if n > 0 {
				end := time.Now()
				diff := end.Sub(start).Microseconds()
				start = end
				if sofar < *samples {
					_ = hist.RecordValue(diff)
					sofar++
				} else {
					log.Printf(
						"loop latency min/avg/max/stddev = %d/%d/%d/%dus p95=%d p99=%d p99.5=%d p99.9=%d p99.99=%d n_buffered=%d",
						int(hist.Min()),
						int(hist.Mean()),
						int(hist.Max()),
						int(hist.StdDev()),
						hist.ValueAtPercentile(95.0),
						hist.ValueAtPercentile(99.0),
						hist.ValueAtPercentile(99.5),
						hist.ValueAtPercentile(99.9),
						hist.ValueAtPercentile(99.99),
						proc.Buffered(),
					)
					hist.Reset()

					sofar = 0
				}

			}
		}
	} else {
		for {
			_, _ = ioc.PollOne()
		}
	}
}
