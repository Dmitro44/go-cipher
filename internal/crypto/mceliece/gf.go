package mceliece

import "fmt"

// Арифметика поля GF(2^m).
//
// Элементы поля — m-битные слова uint16. Сложение в характеристике 2
// совпадает с побитовым XOR, поэтому вычитание не отличается от сложения.
// Умножение реализовано через таблицы логарифмов и антилогарифмов:
// ненулевые элементы поля — это степени образующего элемента z, и
// a*b = z^(log a + log b).

// fieldPolynomials задаёт представление GF(2^m): примитивный многочлен,
// по которому выполняется приведение при умножении.
var fieldPolynomials = map[int]uint16{
	8:  0x11D, // z^8 + z^4 + z^3 + z^2 + 1
	10: 0x409, // z^10 + z^3 + 1
}

// Field — поле GF(2^m) с готовыми таблицами логарифмов.
type Field struct {
	m    int
	size int    // 2^m
	poly uint16 // примитивный многочлен поля
	log  []int  // log[x] — степень z, дающая x; log[0] не используется
	exp  []uint16
}

// NewField строит таблицы поля GF(2^m). Поддерживаются m = 8 и m = 10.
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
		// Таблица удвоена: сумма логарифмов лежит в [0, 2*(size-1)),
		// поэтому брать её по модулю при умножении не нужно.
		exp: make([]uint16, 2*(size-1)),
	}

	// Образующий элемент — сам z; если многочлен примитивен, его степени
	// обходят все ненулевые элементы поля ровно один раз. Таблица
	// заполняется дважды: вторая половина — те же степени, сдвинутые на
	// period = size-1, чтобы индексы сумм логарифмов не брать по модулю.
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

// mulRaw умножает два элемента без таблиц: сдвиги и приведение по
// многочлену поля. Нужен только при построении таблиц.
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

// Add — сложение в GF(2^m), оно же вычитание.
func (f *Field) Add(a, b uint16) uint16 { return a ^ b }

// Mul умножает через таблицы логарифмов.
func (f *Field) Mul(a, b uint16) uint16 {
	if a == 0 || b == 0 {
		return 0
	}
	return f.exp[f.log[a]+f.log[b]]
}

// Square возводит элемент в квадрат.
func (f *Field) Square(a uint16) uint16 {
	if a == 0 {
		return 0
	}
	return f.exp[2*f.log[a]]
}

// Inv возвращает обратный элемент; Inv(0) = 0, вызывающий код не должен
// передавать ноль.
func (f *Field) Inv(a uint16) uint16 {
	if a == 0 {
		return 0
	}
	return f.exp[f.size-1-f.log[a]]
}

// Div делит a на b через таблицы логарифмов.
func (f *Field) Div(a, b uint16) uint16 {
	if a == 0 || b == 0 {
		return 0
	}
	return f.exp[f.log[a]-f.log[b]+f.size-1]
}

// Sqrt вычисляет квадратный корень: в характеристике 2 возведение в
// квадрат — биекция (x^(2^m) = x), поэтому sqrt(x) = x^(2^(m-1)), а в
// логарифмах это деление показателя на 2 по модулю size-1.
func (f *Field) Sqrt(a uint16) uint16 {
	if a == 0 {
		return 0
	}
	n := f.size - 1
	k := f.log[a]
	if k%2 != 0 {
		k += n // n нечётно, поэтому k станет чётным
	}
	return f.exp[k/2]
}

// Elements возвращает все элементы поля в фиксированном порядке 0..2^m-1.
func (f *Field) Elements() []uint16 {
	all := make([]uint16, f.size)
	for i := range all {
		all[i] = uint16(i)
	}
	return all
}
