package mceliece

import "fmt"

var fieldPolynomials = map[int]uint16{
	8:  0x11D,
	10: 0x409,
}

type Field struct {
	m    int
	size int
	poly uint16
	log  []int
	exp  []uint16
}

func NewField(m int) (*Field, error) {
	poly, ok := fieldPolynomials[m]
	if !ok {
		return nil, fmt.Errorf("mceliece: no field polynomial for GF(2^%d)", m)
	}

	size := 1 << m
	f := &Field{
		m:    m,
		size: size,
		poly: poly,
		log:  make([]int, size),
		exp: make([]uint16, 2*(size-1)),
	}

	x := uint16(1)
	period := size - 1
	for i := 0; i < period; i++ {
		if i > 0 && x == 1 {
			return nil, fmt.Errorf("mceliece: polynomial 0x%X is not primitive", poly)
		}
		f.exp[i] = x
		f.exp[i+period] = x
		f.log[x] = i
		x = f.mulRaw(x, 2)
	}
	if x != 1 {
		return nil, fmt.Errorf("mceliece: polynomial 0x%X is not primitive", poly)
	}
	return f, nil
}

func (f *Field) Degree() int { return f.m }

func (f *Field) Size() int { return f.size }

func (f *Field) mulRaw(a, b uint16) uint16 {
	var result uint16
	for b != 0 {
		if b&1 != 0 {
			result ^= a
		}
		b >>= 1
		a <<= 1
		if a&uint16(f.size) != 0 {
			a ^= f.poly
		}
	}
	return result
}

func (f *Field) Add(a, b uint16) uint16 { return a ^ b }

func (f *Field) Mul(a, b uint16) uint16 {
	if a == 0 || b == 0 {
		return 0
	}
	return f.exp[f.log[a]+f.log[b]]
}

func (f *Field) Square(a uint16) uint16 {
	if a == 0 {
		return 0
	}
	return f.exp[2*f.log[a]]
}

func (f *Field) Inv(a uint16) uint16 {
	if a == 0 {
		return 0
	}
	return f.exp[f.size-1-f.log[a]]
}

func (f *Field) Div(a, b uint16) uint16 {
	if a == 0 || b == 0 {
		return 0
	}
	return f.exp[f.log[a]-f.log[b]+f.size-1]
}

func (f *Field) Sqrt(a uint16) uint16 {
	if a == 0 {
		return 0
	}
	n := f.size - 1
	k := f.log[a]
	if k%2 != 0 {
		k += n
	}
	return f.exp[k/2]
}

func (f *Field) Elements() []uint16 {
	all := make([]uint16, f.size)
	for i := range all {
		all[i] = uint16(i)
	}
	return all
}
