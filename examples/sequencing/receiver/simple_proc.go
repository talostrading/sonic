package main

import (
	"log"
	"sort"

	"github.com/csdenboer/sonic"
)

var _ ByteBufferProcessor = &SimpleProcessor{}

type SimpleProcessor struct {
	expected int
	buffer   []struct {
		seq int
		b   []byte
	}
}

func NewSimpleProcessor() *SimpleProcessor {
	p := &SimpleProcessor{expected: 1}
	log.Println("using simple processor")
	return p
}

func (p *SimpleProcessor) Process(
	seq int,
	payload []byte,
	b *sonic.ByteBuffer,
) int {
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

		return 0
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

	return 0
}

func (p *SimpleProcessor) addToBuffer(seq int, payload []byte) (buffered bool) {
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

func (p *SimpleProcessor) walkBuffer() {
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

func (p *SimpleProcessor) Buffered() int {
	return len(p.buffer)
}
