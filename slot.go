package sonic

import "errors"

var ErrNoSpaceLeftForSlot = errors.New(
	"no space left to buffer the given slot",
)

// Slot from the save area. See Save and Discard.
type Slot struct {
	Index  int
	Length int
}

// OffsetSlot ...
//
// Usually used when we reference a few Slots and Discard several of them.
// - Slots that precede a discarded Slot must be offset
// - Slots that follow a discarded Slot must not be offset
func OffsetSlot(offset int, slot Slot) Slot {
	if offset < 0 {
		offset = 0
	}

	if offset > slot.Index {
		offset = slot.Index
	}

	return Slot{
		Index:  slot.Index - offset,
		Length: slot.Length,
	}
}
