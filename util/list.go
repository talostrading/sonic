package util

import "errors"

type listNode[T comparable] struct {
	v    T
	next *listNode[T]
}

// List of singly-linked nodes.
type List[T comparable] struct {
	head *listNode[T]
	n    int
}

func NewList[T comparable]() *List[T] {
	l := &List[T]{}
	return l
}

func (l *List[T]) Add(v T) {
	node := &listNode[T]{
		v:    v,
		next: nil,
	}

	if l.head == nil {
		l.head = node
	} else {
		last := l.head
		for last.next != nil {
			last = last.next
		}
		last.next = node
	}

	l.n++
}

var ErrOutOfBounds = errors.New("index out of bounds")

func (l *List[T]) At(ix int) T {
	if ix >= l.Size() {
		panic(ErrOutOfBounds)
	}

	p := l.head
	for i := 0; i < ix; i++ {
		p = p.next
	}
	return p.v
}

func (l *List[T]) Exists(v T) bool {
	p := l.head
	for p != nil {
		if p.v == v {
			return true
		}
		p = p.next
	}
	return false
}

func (l *List[T]) RemoveValue(v T) bool {
	var (
		prev *listNode[T] = nil
		cur               = l.head
	)

	for cur != nil {
		if cur.v == v {
			if prev == nil {
				l.head = cur.next
			} else {
				prev.next = cur.next
			}

			l.n--

			return true
		}
	}
	return false
}

func (l *List[T]) RemoveIndex(ix int) (v T) {
	if ix >= l.Size() {
		panic(ErrOutOfBounds)
	}

	var (
		prev *listNode[T] = nil
		cur               = l.head
	)
	for i := 0; i < ix; i++ {
		prev = cur
		cur = cur.next
	}

	v = cur.v

	if prev == nil {
		l.head = cur.next
	} else {
		prev.next = cur.next
	}

	l.n--

	return
}

func (l *List[T]) Size() int {
	return l.n
}

func (l *List[T]) Iterate(fn func(v *T)) {
	p := l.head
	for p != nil {
		fn(&p.v)
		p = p.next
	}
}
