package sonic

import (
	"bytes"
	"log"
	"math/rand"
	"testing"
	"time"
)

func TestSlotOffsetter1(t *testing.T) {
	// slots of same length, save all then discard all

	s := NewSlotOffsetter(1024)
	b := NewByteBuffer()

	slots := make(map[byte]Slot)

	letters := []byte("abcde")
	b.Write(letters)
	b.Commit(len(letters))
	for i := 0; i < len(letters); i++ {
		before := b.Save(1)
		after, err := s.Add(before)
		if err != nil {
			t.Fatal(err)
		}
		if before.Index != after.Index {
			t.Fatal("wrong add")
		}
		slots[letters[i]] = after
	}

	discarded := 0
	fn := func(letter byte) {
		slot := s.Offset(slots[letter])
		if !bytes.Equal(b.SavedSlot(slot), []byte{letter}) {
			t.Fatal("wrong slot")
		}
		discarded += b.Discard(slot)
		if s.tree.Sum() != discarded {
			t.Fatal("wrong offsets")
		}
	}
	fn('c')
	fn('a')
	fn('b')
	fn('e')
	fn('d')
}

func TestSlotOffsetter2(t *testing.T) {
	// slots of different lengths, save all then discard all

	s := NewSlotOffsetter(1024)
	b := NewByteBuffer()

	slots := make(map[byte]Slot)

	letters := [][]byte{
		[]byte("aaaa"),
		[]byte("bb"),
		[]byte("ccccc"),
		[]byte("d"),
		[]byte("eee"),
	}
	for i := 0; i < len(letters); i++ {
		b.Write(letters[i])
		b.Commit(len(letters[i]))

		before := b.Save(len(letters[i]))
		after, err := s.Add(before)
		if err != nil {
			t.Fatal(err)
		}
		if before.Index != after.Index {
			t.Fatal("wrong add")
		}
		slots[letters[i][0]] = after
	}

	discarded := 0
	fn := func(letter byte) {
		slot := s.Offset(slots[letter])

		var compareTo []byte
		for _, ls := range letters {
			if ls[0] == letter {
				compareTo = ls
			}
		}
		if !bytes.Equal(b.SavedSlot(slot), compareTo) {
			t.Fatal("wrong slot")
		}

		discarded += b.Discard(slot)
		if s.tree.Sum() != discarded {
			t.Fatal("wrong offsets")
		}
	}
	fn('c')
	fn('a')
	fn('b')
	fn('e')
	fn('d')
}

func TestSlotOffsetter3(t *testing.T) {
	// slots of same lengths, save and discard randomly

	s := NewSlotOffsetter(1024)
	b := NewByteBuffer()
	slots := make(map[byte]Slot)

	add := func(letter byte) {
		b.Write([]byte{letter})
		b.Commit(1)

		var err error
		slots[letter], err = s.Add(b.Save(1))
		if err != nil {
			t.Fatal(err)
		}
	}

	pop := func(letter byte) {
		slot := s.Offset(slots[letter])
		if !bytes.Equal(b.SavedSlot(slot), []byte{letter}) {
			t.Fatal("wrong slot")
		}
		b.Discard(slot)
		delete(slots, letter)
	}

	add('a')
	add('b')
	add('c')

	pop('b')

	add('d')
	add('e')

	pop('a')
	pop('c')
	pop('e')

	_, ok := slots['d']
	if !ok || len(slots) != 1 {
		t.Fatal("d should be the only letter left")
	}

	// d remains, now add f g h i j k l m n
	sum := s.tree.Sum()
	for _, letter := range []byte("fghijklmn") {
		add(letter)
	}
	if sum != s.tree.Sum() {
		t.Fatal("prefix sum should have not changed")
	}

	pop('g')
	pop('i')
	pop('k')

	// now just pop the rest
	var remaining []byte
	for letter := range slots {
		remaining = append(remaining, letter)
	}
	for _, letter := range remaining {
		pop(letter)
	}

	// sanity check
	if len(slots) != 0 || b.SaveLen() != 0 {
		t.Fatal("there should be no slots left")
	}
}

func TestSlotOffsetter4(t *testing.T) {
	// like test3 but slots of different lengths, save and discard randomly

	s := NewSlotOffsetter(1024)
	b := NewByteBuffer()
	slots := make(map[byte]Slot)
	lengths := make(map[byte]int)

	add := func(letter byte, n int) {
		var letters []byte
		for i := 0; i < n; i++ {
			letters = append(letters, letter)
		}

		b.Write(letters)
		b.Commit(len(letters))

		var err error
		slots[letter], err = s.Add(b.Save(len(letters)))
		if err != nil {
			t.Fatal(err)
		}

		lengths[letter] = len(letters)
	}

	pop := func(letter byte) {
		slot := s.Offset(slots[letter])

		var toCompare []byte
		for i := 0; i < lengths[letter]; i++ {
			toCompare = append(toCompare, letter)
		}
		if !bytes.Equal(b.SavedSlot(slot), toCompare) {
			t.Fatal("wrong slot")
		}
		b.Discard(slot)
		delete(slots, letter)
		delete(lengths, letter)
	}

	add('a', 4)
	add('b', 10)
	add('c', 20)

	pop('b')

	add('d', 2)
	add('e', 3)

	pop('a')
	pop('c')
	pop('e')

	_, ok := slots['d']
	if !ok || len(slots) != 1 {
		t.Fatal("d should be the only letter left")
	}
	length, ok := lengths['d']
	if !ok || len(lengths) != 1 || length != 2 {
		t.Fatal("wrong length for d")
	}

	// d remains, now add f g h i j k l m n
	sum := s.tree.Sum()
	for _, letter := range []byte("fghijklmn") {
		add(letter, rand.Int()%100+1)
	}
	if sum != s.tree.Sum() {
		t.Fatal("prefix sum should have not changed")
	}

	pop('g')
	pop('i')
	pop('k')

	// now just pop the rest
	var remaining []byte
	for letter := range slots {
		remaining = append(remaining, letter)
	}
	for _, letter := range remaining {
		pop(letter)
	}

	// sanity check
	if len(slots) != 0 || b.SaveLen() != 0 || len(lengths) != 0 {
		t.Fatal("there should be no slots left")
	}
}

func TestOffsetterRandom(t *testing.T) {
	s := NewSlotOffsetter(1024 * 1024)
	b := NewByteBuffer()
	slots := make(map[byte]Slot)
	lengths := make(map[byte]int)

	add := func(letter byte, n int) {
		var letters []byte
		for i := 0; i < n; i++ {
			letters = append(letters, letter)
		}

		b.Write(letters)
		b.Commit(len(letters))

		var err error
		slots[letter], err = s.Add(b.Save(len(letters)))
		if err != nil {
			t.Fatal(err)
		}

		lengths[letter] = len(letters)
	}

	pop := func(letter byte) {
		if _, ok := slots[letter]; !ok {
			return
		}

		slot := s.Offset(slots[letter])

		var toCompare []byte
		for i := 0; i < lengths[letter]; i++ {
			toCompare = append(toCompare, letter)
		}
		if !bytes.Equal(b.SavedSlot(slot), toCompare) {
			t.Fatal("wrong slot")
		}
		b.Discard(slot)
		delete(slots, letter)
		delete(lengths, letter)
	}

	letters := []byte("abcdefghijklmnopqrstuvwxyz")

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	iterations := 0
	start := time.Now()
	for time.Since(start).Seconds() < 30 {
		ix := 0
		for ix < len(letters) {
			n := rand.Int()%len(letters[ix:]) + 1
			for _, letter := range letters[ix : ix+n] {
				add(letter, rand.Int()%100+1)
			}
			ix += n

			for _, letter := range letters {
				zeroOne := rand.Int() % 2
				if zeroOne == 1 {
					pop(letter)
				}
			}

		}
		// pop the rest
		var remaining []byte
		for letter := range slots {
			remaining = append(remaining, letter)
		}
		for _, letter := range remaining {
			pop(letter)
		}

		// sanity check
		if len(slots) != 0 || b.SaveLen() != 0 || len(lengths) != 0 {
			t.Fatal("there should be no slots left")
		}

		// This is what callers should also do after all slots are removed
		// from the save area.
		s.Reset()

		iterations++
	}

	log.Printf("slot offset random test iterations=%d", iterations)
}
