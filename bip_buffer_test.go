package sonic

import (
	"bytes"
	"testing"
)

func TestBipBufferClaim(t *testing.T) {
	buf := NewBipBuffer(8)
	{
		b := buf.Claim(3)
		if len(b) != 3 {
			t.Fatal("wrong reserved slice")
		}
	}
	{
		b := buf.Claim(8)
		if len(b) != 8 {
			t.Fatal("wrong reserved slice")
		}
	}
	{
		b := buf.Claim(1024)
		if len(b) != 8 {
			t.Fatal("wrong reserved slice")
		}
	}
}

func TestBipBufferData(t *testing.T) {
	buf := NewBipBuffer(3)
	if buf.Data() != nil {
		t.Fatal("wrong data slice")
	}
}

func TestBipBufferDataUncommitted(t *testing.T) {
	buf := NewBipBuffer(3)
	buf.Claim(2)
	if buf.Data() != nil {
		t.Fatal("wrong data slice")
	}
}

func TestBipBufferOverClaim(t *testing.T) {
	buf := NewBipBuffer(3)
	if buf.Claimed() != 0 {
		t.Fatal("0 should be claimed")
	}
	b := buf.Claim(4)
	if len(b) != 3 {
		t.Fatal("wrong claim")
	}
	if buf.Claimed() != 3 {
		t.Fatal("wrong claim")
	}
}

func TestBipBufferCommit(t *testing.T) {
	buf := NewBipBuffer(4)
	b := buf.Claim(3)
	b[0] = 7
	b[1] = 22
	b[2] = 218
	if buf.Committed() != 0 {
		t.Fatal("wrong committed")
	}
	if buf.Claimed() != 3 {
		t.Fatal("wrong committed")
	}
	buf.Commit(3)
	if buf.Committed() != 3 {
		t.Fatal("wrong committed")
	}
	if buf.Claimed() != 0 {
		t.Fatal("wrong committed")
	}
	b = buf.Data()
	if !bytes.Equal(b, []byte{7, 22, 218}) {
		t.Fatal("wrong data")
	}
}

func TestBipBufferClaimAll(t *testing.T) {
	buf := NewBipBuffer(4)
	buf.Claim(4)
	buf.Commit(4)
	if buf.Claim(1) != nil {
		t.Fatal("should not be able to claim anything")
	}
}

func TestBipBufferConsume(t *testing.T) {
	buf := NewBipBuffer(4)
	{
		b := buf.Claim(4)
		b[0] = 7
		b[1] = 22
		b[2] = 218
		b[3] = 56
	}
	buf.Commit(4)
	buf.Consume(2)
	{
		b := buf.Data()
		if !bytes.Equal(b, []byte{218, 56}) {
			t.Fatal("wrong data")
		}
	}
	buf.Consume(1)
	{
		b := buf.Data()
		if !bytes.Equal(b, []byte{56}) {
			t.Fatal("wrong data")
		}
	}
}

func TestBipBufferClaimAfterWrapping(t *testing.T) {
	buf := NewBipBuffer(4)
	{
		b := buf.Claim(4)
		b[0] = 7
		b[1] = 22
		b[2] = 218
		b[3] = 56
	}
	buf.Commit(4)
	buf.Consume(2)
	{
		b := buf.Claim(4)
		if len(b) != 2 {
			t.Fatal("wrong claim")
		}
		b[0] = 49
		b[1] = 81
	}
	buf.Commit(2)
	{
		b := buf.Data()
		if !bytes.Equal(b, []byte{218, 56}) {
			t.Fatal("wrong data")
		}
	}
	buf.Consume(2)
	{
		b := buf.Data()
		if !bytes.Equal(b, []byte{49, 81}) {
			t.Fatal("wrong data")
		}
	}
}

func TestBipBufferReset(t *testing.T) {
	buf := NewBipBuffer(4)
	{
		b := buf.Claim(4)
		b[0] = 7
		b[1] = 22
		b[2] = 218
		b[3] = 56
	}
	if buf.Claimed() != 4 {
		t.Fatal("wrong claimed")
	}
	buf.Commit(4)
	if buf.Claimed() != 0 {
		t.Fatal("wrong claimed")
	}
	buf.Reset()
	if buf.Committed() != 0 {
		t.Fatal("wrong committed")
	}
}

func TestBipBufferClaimSameChunk(t *testing.T) {
	buf := NewBipBuffer(4)
	{
		b := buf.Claim(4)
		b[0] = 7
		b[1] = 22
		b[2] = 218
		b[3] = 56
	}
	{
		b := buf.Claim(4)
		if !bytes.Equal(b, []byte{7, 22, 218, 56}) {
			t.Fatal("wrong claim")
		}
		b[0] = 7 + 1
		b[1] = 22 + 1
		b[2] = 218 + 1
		b[3] = 56 + 1
	}
	{
		b := buf.Claim(4)
		if !bytes.Equal(b, []byte{7 + 1, 22 + 1, 218 + 1, 56 + 1}) {
			t.Fatal("wrong claim")
		}
	}
}
