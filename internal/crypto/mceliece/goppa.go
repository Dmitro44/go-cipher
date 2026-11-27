package mceliece

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
)

// Построение бинарного кода Гоппы над GF(2^m).
//
// Код Гоппы с многочленом g(z) степени t и опорными элементами a_0..a_(n-1)
// задаётся проверочной матрицей H размера m*t × n: столбец i — это
// разложение 1/(z - a_i) mod g по базису 1, z, ..., z^(t-1), записанное
// m-битными коэффициентами. Код исправляет до t ошибок.

// Params — параметры кода: поле GF(2^m) и степень t многочлена Гоппы.
type Params struct {
	M int // поле GF(2^m)
	T int // степень g(z)
	N int // длина кода n = 2^m
	K int // размерность k = n - m*t
}

// NewParams проверяет параметры и вычисляет n и k.
func NewParams(m, t int) (Params, error) {
	if _, ok := fieldPolynomials[m]; !ok {
		return Params{}, fmt.Errorf("mceliece: no field polynomial for GF(2^%d)", m)
	}
	if t < 1 {
		return Params{}, fmt.Errorf("mceliece: Goppa degree must be positive, got %d", t)
	}
	n := 1 << m
	k := n - m*t
	if k <= 0 {
		return Params{}, fmt.Errorf("mceliece: n - m*t = %d must be positive", k)
	}
	return Params{M: m, T: t, N: n, K: k}, nil
}

// GoppaCode — код Гоппы в систематическом виде [I_k | A^T]. Support хранит
// опорный элемент каждого столбца после перестановок систематизации.
type GoppaCode struct {
	Params  Params
	Field   *Field
	G       Poly
	Support []uint16
	Matrix  *Matrix
}

// NewGoppaCode строит код Гоппы по случайному неприводимому многочлену и
// случайному упорядочению всех элементов поля.
func NewGoppaCode(params Params, random io.Reader) (*GoppaCode, error) {
	field, err := NewField(params.M)
	if err != nil {
		return nil, err
	}
	g, err := RandomIrreduciblePolynomial(field, params.T, random)
	if err != nil {
		return nil, err
	}
	support, err := randomPermutation(field.Elements(), random)
	if err != nil {
		return nil, err
	}
	return BuildGoppaCode(field, g, support)
}

// BuildGoppaCode строит систематическую порождающую матрицу по уже
// заданным g и опорным элементам.
func BuildGoppaCode(field *Field, g Poly, support []uint16) (*GoppaCode, error) {
	params, err := NewParams(field.Degree(), Deg(g))
	if err != nil {
		return nil, err
	}
	if len(support) != params.N {
		return nil, fmt.Errorf("mceliece: support length %d, want %d", len(support), params.N)
	}

	h, err := parityCheckMatrix(field, g, support)
	if err != nil {
		return nil, err
	}
	// Переставляем столбцы так, чтобы правый блок m*t столбцов стал
	// единичной матрицей; перестановка запоминается в support.
	if err := systematicForm(h, support); err != nil {
		return nil, err
	}

	return &GoppaCode{
		Params:  params,
		Field:   field,
		G:       g,
		Support: support,
		Matrix:  generatorMatrix(h, params.K),
	}, nil
}

// RandomIrreduciblePolynomial возвращает случайный приведённый неприводимый
// многочлен степени t.
func RandomIrreduciblePolynomial(field *Field, t int, random io.Reader) (Poly, error) {
	if t < 1 {
		return nil, fmt.Errorf("mceliece: degree must be positive, got %d", t)
	}
	g := make(Poly, t+1)
	for attempt := 0; attempt < 100000; attempt++ {
		for i := 0; i < t; i++ {
			a, err := randomElement(field, random)
			if err != nil {
				return nil, err
			}
			g[i] = a
		}
		g[t] = 1
		if isIrreducible(field, g, t) {
			return g, nil
		}
	}
	return nil, errors.New("mceliece: could not find an irreducible polynomial")
}

// isIrreducible проверяет неприводимость критерием Рабина: для каждого
// собственного делителя 2^(m*i) (i = 1..t/2) должно быть
//
//	gcd(g(z), z^(2^(m*i)) - z) = 1,
//
// а на последнем шаге z^(2^(m*t)) = z (mod g).
func isIrreducible(field *Field, g Poly, t int) bool {
	x := Poly{0, 1}
	power := field.frobenius(x, g) // x^(2^(m*1))
	for i := 1; i <= t; i++ {
		if i > 1 {
			power = field.frobenius(power, g)
		}
		if i <= t/2 {
			difference := field.PolyAdd(power, x)
			if Deg(field.PolyGCD(difference, g)) != 0 {
				return false
			}
		}
	}
	return field.PolyEqual(power, field.PolyMod(x, g))
}

// PolyEqual сравнивает многочлены после нормализации.
func (f *Field) PolyEqual(a, b Poly) bool {
	a, b = f.PolyNormalize(a), f.PolyNormalize(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// parityCheckMatrix строит проверочную матрицу m*t × n.
func parityCheckMatrix(field *Field, g Poly, support []uint16) (*Matrix, error) {
	m := field.Degree()
	t := Deg(g)
	h := NewMatrix(m*t, len(support))

	for column, a := range support {
		// Столбец — коэффициенты 1/(z - a) mod g, то есть обратного к
		// линейному многочлену z + a (в характеристике 2 z - a = z + a).
		inverse, err := field.PolyInv(Poly{a, 1}, g)
		if err != nil {
			return nil, fmt.Errorf("mceliece: g has root %d: %w", a, err)
		}
		for row := 0; row < t; row++ {
			var coefficient uint16
			if row < len(inverse) {
				coefficient = inverse[row]
			}
			for bit := 0; bit < m; bit++ {
				if coefficient&(1<<uint(bit)) != 0 {
					h.Set(row*m+bit, column, true)
				}
			}
		}
	}
	return h, nil
}

// systematicForm приводит H к виду [A | I_(m*t)] перестановками столбцов.
// Вместе со столбцами переставляются опорные элементы: координата j кода
// по-прежнему соответствует support[j].
func systematicForm(h *Matrix, support []uint16) error {
	rows, n := h.Rows, h.Cols
	k := n - rows
	used := make([]bool, n)

	for r := 0; r < rows; r++ {
		// Ищем ведущий столбец: сначала в правом блоке, который станет
		// единичной матрицей, затем в левом — тогда столбец из левого
		// блока меняется местами с ещё свободным правым.
		pivot := -1
		for j := k; j < n; j++ {
			if !used[j] && h.At(r, j) {
				pivot = j
				break
			}
		}
		if pivot < 0 {
			for j := 0; j < k && pivot < 0; j++ {
				if used[j] || !h.At(r, j) {
					continue
				}
				for c := k; c < n; c++ {
					if used[c] {
						continue
					}
					h.SwapCols(j, c)
					support[j], support[c] = support[c], support[j]
					pivot = c
					break
				}
			}
		}
		if pivot < 0 {
			return errors.New("mceliece: parity-check matrix is rank deficient")
		}

		target := k + r
		if pivot != target {
			h.SwapCols(pivot, target)
			support[pivot], support[target] = support[target], support[pivot]
		}
		used[target] = true

		for row := 0; row < rows; row++ {
			if row != r && h.At(row, target) {
				h.XorRow(row, r)
			}
		}
	}
	return nil
}

// generatorMatrix строит G = [I_k | A^T] по H = [A | I].
func generatorMatrix(h *Matrix, k int) *Matrix {
	n := h.Cols
	g := NewMatrix(k, n)
	for row := 0; row < k; row++ {
		g.Set(row, row, true)
		for r := 0; r < h.Rows; r++ {
			if h.At(r, row) {
				g.Set(row, k+r, true)
			}
		}
	}
	return g
}

// Decoder возвращает декодер Паттерсона для этого кода.
func (c *GoppaCode) Decoder() *Decoder {
	return &Decoder{Field: c.Field, Params: c.Params, G: c.G, Support: c.Support}
}

func randomBelow(random io.Reader, n int) (int, error) {
	value, err := rand.Int(random, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func randomElement(field *Field, random io.Reader) (uint16, error) {
	index, err := randomBelow(random, field.Size())
	if err != nil {
		return 0, err
	}
	return uint16(index), nil
}

// randomPermutation перемешивает все элементы поля Фишером — Йетсом.
func randomPermutation(elements []uint16, random io.Reader) ([]uint16, error) {
	order := append([]uint16(nil), elements...)
	for i := len(order) - 1; i > 0; i-- {
		j, err := randomBelow(random, i+1)
		if err != nil {
			return nil, err
		}
		order[i], order[j] = order[j], order[i]
	}
	return order, nil
}
