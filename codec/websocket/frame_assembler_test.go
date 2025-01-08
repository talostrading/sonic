package websocket

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFrameAssemblerEmpty(t *testing.T) {
	assembler := NewFrameAssembler()
	assert.NotNil(t, assembler)

	slices := assembler.Slices()
	assert.Len(t, slices, 0)

	reassembled := assembler.Reassemble()
	assert.Equal(t, 0, len(reassembled))
}

func TestFrameAssemblerSingle(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03}
	assembler := NewFrameAssembler(payload)

	slices := assembler.Slices()
	assert.Len(t, slices, 1)
	assert.Equal(t, payload, slices[0])

	reassembled := assembler.Reassemble()
	assert.Equal(t, payload, reassembled)
}

func TestFrameAssemblerMulti(t *testing.T) {
	payload1 := []byte{0x01, 0x02}
	payload2 := []byte{0x03, 0x04, 0x05}
	payload3 := []byte{0x06}

	assembler := NewFrameAssembler(payload1, payload2, payload3)

	slices := assembler.Slices()
	assert.Len(t, slices, 3)
	assert.Equal(t, payload1, slices[0])
	assert.Equal(t, payload2, slices[1])
	assert.Equal(t, payload3, slices[2])

	reassembled := assembler.Reassemble()
	want := bytes.Join([][]byte{payload1, payload2, payload3}, nil)
	assert.Equal(t, want, reassembled)
}

func TestFrameAssemblerAppend(t *testing.T) {
	assembler := NewFrameAssembler([]byte{0x01}, []byte{0x02})
	assembler.Append([]byte{0x03, 0x04})

	slices := assembler.Slices()
	assert.Len(t, slices, 3)

	want := []byte{0x01, 0x02, 0x03, 0x04}
	reassembled := assembler.Reassemble()
	assert.Equal(t, want, reassembled)
}