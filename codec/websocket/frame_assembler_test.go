package websocket

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFrameAssemblerEmpty(t *testing.T) {
	assert := assert.New(t)

	assembler := NewFrameAssembler()
	assert.NotNil(assembler)

	slices := assembler.Slices()
	assert.Len(slices, 0)

	reassembled := assembler.Reassemble()
	assert.Equal(0, len(reassembled))
}

func TestFrameAssemblerSingle(t *testing.T) {
	assert := assert.New(t)

	payload := []byte{0x01, 0x02, 0x03}
	assembler := NewFrameAssembler(payload)

	slices := assembler.Slices()
	assert.Len(slices, 1)
	assert.Equal(payload, slices[0])

	reassembled := assembler.Reassemble()
	assert.Equal(payload, reassembled)
}

func TestFrameAssemblerMulti(t *testing.T) {
	assert := assert.New(t)

	payload1 := []byte{0x01, 0x02}
	payload2 := []byte{0x03, 0x04, 0x05}
	payload3 := []byte{0x06}

	assembler := NewFrameAssembler(payload1, payload2, payload3)

	slices := assembler.Slices()
	assert.Len(slices, 3)
	assert.Equal(payload1, slices[0])
	assert.Equal(payload2, slices[1])
	assert.Equal(payload3, slices[2])

	reassembled := assembler.Reassemble()
	want := bytes.Join([][]byte{payload1, payload2, payload3}, nil)
	assert.Equal(want, reassembled)
}

func TestFrameAssemblerReassembleInto(t *testing.T) {
	assert := assert.New(t)

	payload1 := []byte{0x01, 0x02}
	payload2 := []byte{0x03, 0x04, 0x05}
	payload3 := []byte{0x06}

	assembler := NewFrameAssembler(payload1, payload2, payload3)

	want := make([]byte, 2)

	smallBuffer := make([]byte, 2)
	assert.False(assembler.ReassembleInto(smallBuffer))
	assert.Equal(want, smallBuffer)

	want = bytes.Join([][]byte{payload1, payload2, payload3}, nil)

	reassembleBuffer := make([]byte, len(want))
	assert.True(assembler.ReassembleInto(reassembleBuffer))
	assert.Equal(want, reassembleBuffer)
}

func TestFrameAssemblerAppend(t *testing.T) {
	assert := assert.New(t)

	assembler := NewFrameAssembler([]byte{0x01}, []byte{0x02})
	assembler.Append([]byte{0x03, 0x04})

	slices := assembler.Slices()
	assert.Len(slices, 3)

	want := []byte{0x01, 0x02, 0x03, 0x04}
	reassembled := assembler.Reassemble()
	assert.Equal(want, reassembled)
}