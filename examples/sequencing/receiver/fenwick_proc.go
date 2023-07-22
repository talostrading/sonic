package main

import (
	"fmt"
	"log"

	"github.com/talostrading/sonic"
)

var _ ByteBufferProcessor = &FenwickProcessor{}

type FenwickProcessor struct {
	expected  int
	sequencer *sonic.SlotSequencer
}

func NewFenwickProcessor() *FenwickProcessor {
	p := &FenwickProcessor{
		expected:  1,
		sequencer: sonic.NewSlotSequencer(*maxSlots, *bufSize),
	}
	log.Printf(
		"created slot sequencer max_slots=%d buf_size=%d",
		*maxSlots,
		*bufSize,
	)
	log.Println("using fenwick processor")
	return p
}

func (p *FenwickProcessor) Process(
	seq int,
	payload []byte,
	b *sonic.ByteBuffer,
) int {
	discarded := 0

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
		return 0
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
		discarded = p.walkBuffer(b)
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

	return discarded
}

func (p *FenwickProcessor) walkBuffer(b *sonic.ByteBuffer) int {
	discarded := 0
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

		discarded++
	}
	return discarded
}

func (p *FenwickProcessor) addToBuffer(
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

func (p *FenwickProcessor) Buffered() int {
	return p.sequencer.Size()
}
