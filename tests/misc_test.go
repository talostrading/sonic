package tests

import "testing"

var (
	_ IBase = &Base{}
	_ IBase = &Static[IBase]{}
	_ IBase = &Dynamic{}
	_ IBase = &DynamicEmbed{}
)

type IBase interface {
	Foo() int
}

type Base struct{}

func (b *Base) Foo() int { return 0 }

type BaseV struct{}

func (b BaseV) Foo() int { return 0 }

type Static[BaseT IBase] struct {
	base BaseT
}

func NewStatic[BaseT IBase]() *Static[BaseT] {
	return &Static[BaseT]{}
}

func (s *Static[BaseT]) Foo() int { return s.base.Foo() }

type Dynamic struct {
	base IBase
}

func NewDynamic() *Dynamic {
	d := &Dynamic{}
	d.base = &Base{}
	return d
}

func (d *Dynamic) Foo() int { return d.base.Foo() }

type DynamicEmbed struct {
	IBase
}

func NewDynamicEmbed() *DynamicEmbed {
	d := &DynamicEmbed{}
	d.IBase = &Base{}
	return d
}

func BenchmarkDynamicEmbedPolymorphismEmbed(b *testing.B) {
	d := NewDynamicEmbed()
	for i := 0; i < b.N; i++ {
		d.Foo()
	}
}

func BenchmarkStaticPolymorphism1(b *testing.B) {
	// Fake news, there is a dynamic dispatch here when you pass the pointer.
	s := NewStatic[*Base]()
	for i := 0; i < b.N; i++ {
		s.Foo()
	}
}

func BenchmarkStaticPolymorphism2(b *testing.B) {
	// Also fake news, there is a dynamic dispatch here when you pass the pointer.
	s := NewStatic[BaseV]()
	for i := 0; i < b.N; i++ {
		s.Foo()
	}
}

func BenchmarkDynamicPolymorphism(b *testing.B) {
	d := NewDynamic()
	for i := 0; i < b.N; i++ {
		d.Foo()
	}
}
