package mceliece

import (
	"errors"
	"slices"
)

// Двоичные матрицы — так задаются порождающая матрица кода и ключевые
// матрицы Мак-Элиса. Бит (i, j) лежит в слове bits[i*stride + j/64], номер
// бита внутри слова — j%64. Векторы — те же битовые наборы без размерности.

var ErrMatrixShape = errors.New("mceliece: matrix dimensions do not match")

// Matrix — двоичная матрица Rows × Cols.
type Matrix struct {
	Rows, Cols int
	stride     int
	bits       []uint64
}

// NewMatrix создаёт нулевую матрицу.
func NewMatrix(rows, cols int) *Matrix {
	stride := (cols + 63) / 64
	return &Matrix{
		Rows:   rows,
		Cols:   cols,
		stride: stride,
		bits:   make([]uint64, rows*stride),
	}
}

// At возвращает бит (i, j).
func (m *Matrix) At(i, j int) bool {
	return m.bits[i*m.stride+j/64]&(1<<uint(j%64)) != 0
}

// Set записывает бит (i, j).
func (m *Matrix) Set(i, j int, value bool) {
	mask := uint64(1) << uint(j%64)
	if value {
		m.bits[i*m.stride+j/64] |= mask
	} else {
		m.bits[i*m.stride+j/64] &^= mask
	}
}

// XorRow прибавляет строку src к строке dst (в GF(2) сложение — XOR).
func (m *Matrix) XorRow(dst, src int) {
	d, s := dst*m.stride, src*m.stride
	for i := 0; i < m.stride; i++ {
		m.bits[d+i] ^= m.bits[s+i]
	}
}

// SwapRows меняет строки местами.
func (m *Matrix) SwapRows(a, b int) {
	for i := 0; i < m.stride; i++ {
		m.bits[a*m.stride+i], m.bits[b*m.stride+i] = m.bits[b*m.stride+i], m.bits[a*m.stride+i]
	}
}

// SwapCols меняет столбцы местами. При систематизации матрицы такие
// перестановки запоминаются в упорядочении опорных элементов кода.
func (m *Matrix) SwapCols(a, b int) {
	for i := 0; i < m.Rows; i++ {
		av, bv := m.At(i, a), m.At(i, b)
		if av != bv {
			m.Set(i, a, bv)
			m.Set(i, b, av)
		}
	}
}

// Mul умножает матрицу на матрицу: результат Rows × o.Cols.
func (m *Matrix) Mul(o *Matrix) (*Matrix, error) {
	if m.Cols != o.Rows {
		return nil, ErrMatrixShape
	}
	product := NewMatrix(m.Rows, o.Cols)
	for i := 0; i < m.Rows; i++ {
		for a := 0; a < m.Cols; a++ {
			if !m.At(i, a) {
				continue
			}
			for j := 0; j < o.Cols; j++ {
				if o.At(a, j) {
					product.Set(i, j, !product.At(i, j))
				}
			}
		}
	}
	return product, nil
}

// MulVector умножает вектор-строку v длины Rows на матрицу. Результат —
// битовый вектор длины Cols.
func (m *Matrix) MulVector(v []uint64) []uint64 {
	result := make([]uint64, (m.Cols+63)/64)
	for i := 0; i < m.Rows; i++ {
		if v[i/64]&(1<<uint(i%64)) == 0 {
			continue
		}
		for j := 0; j < m.stride; j++ {
			result[j] ^= m.bits[i*m.stride+j]
		}
	}
	return result
}

// Rank вычисляет ранг матрицы над GF(2) методом Гаусса.
func (m *Matrix) Rank() int {
	work := m.Clone()
	rank := 0
	for row := 0; row < work.Rows; row++ {
		pivotCol := -1
		for col := 0; col < work.Cols; col++ {
			if work.At(row, col) {
				pivotCol = col
				break
			}
		}
		if pivotCol < 0 {
			continue
		}
		rank++
		// Обнуляем ведущий бит ниже по столбцу.
		for r := row + 1; r < work.Rows; r++ {
			if work.At(r, pivotCol) {
				work.XorRow(r, row)
			}
		}
	}
	return rank
}

// Invert обращает квадратную матрицу методом Гаусса — Жордана. Второй
// результат — false, если матрица вырождена.
func (m *Matrix) Invert() (*Matrix, bool) {
	if m.Rows != m.Cols {
		return nil, false
	}
	n := m.Rows
	work := m.Clone()
	inv := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		inv.Set(i, i, true)
	}

	for col := 0; col < n; col++ {
		pivot := -1
		for row := col; row < n; row++ {
			if work.At(row, col) {
				pivot = row
				break
			}
		}
		if pivot < 0 {
			return nil, false
		}
		if pivot != col {
			work.SwapRows(pivot, col)
			inv.SwapRows(pivot, col)
		}
		for row := 0; row < n; row++ {
			if row != col && work.At(row, col) {
				work.XorRow(row, col)
				inv.XorRow(row, col)
			}
		}
	}
	return inv, true
}

// Clone возвращает независимую копию.
func (m *Matrix) Clone() *Matrix {
	clone := NewMatrix(m.Rows, m.Cols)
	copy(clone.bits, m.bits)
	return clone
}

// Equal сравнивает размеры и все биты.
func (m *Matrix) Equal(o *Matrix) bool {
	return m.Rows == o.Rows && m.Cols == o.Cols && slices.Equal(m.bits, o.bits)
}

// MarshalBinary записывает матрицу построчно, старший бит вперёд; каждая
// строка занимает (Cols+7)/8 байт.
func (m *Matrix) MarshalBinary() []byte {
	rowBytes := (m.Cols + 7) / 8
	out := make([]byte, m.Rows*rowBytes)
	for i := 0; i < m.Rows; i++ {
		row := make([]uint64, (m.Cols+63)/64)
		copy(row, m.bits[i*m.stride:(i+1)*m.stride])
		copy(out[i*rowBytes:(i+1)*rowBytes], bitsToBytes(row, m.Cols))
	}
	return out
}

// UnmarshalMatrix читает матрицу, записанную MarshalBinary.
func UnmarshalMatrix(rows, cols int, data []byte) (*Matrix, error) {
	rowBytes := (cols + 7) / 8
	if rows <= 0 || cols <= 0 || len(data) != rows*rowBytes {
		return nil, ErrMatrixShape
	}
	matrix := NewMatrix(rows, cols)
	for i := 0; i < rows; i++ {
		row := bitsFromBytes(data[i*rowBytes:(i+1)*rowBytes], cols)
		copy(matrix.bits[i*matrix.stride:(i+1)*matrix.stride], row)
	}
	return matrix, nil
}

// newVector создаёт нулевой битовый вектор длины n.
func newVector(n int) []uint64 { return make([]uint64, (n+63)/64) }

func vectorBit(v []uint64, i int) bool {
	return v[i/64]&(1<<uint(i%64)) != 0
}

func setVectorBit(v []uint64, i int) {
	v[i/64] |= 1 << uint(i%64)
}

// bitsFromBytes переводит байты в битовый вектор: бит i — это бит
// (7 - i%8) байта i/8, то есть старший бит вперёд.
func bitsFromBytes(data []byte, n int) []uint64 {
	bits := make([]uint64, (n+63)/64)
	for i := 0; i < n && i < len(data)*8; i++ {
		if data[i/8]&(0x80>>uint(i%8)) != 0 {
			bits[i/64] |= 1 << uint(i%64)
		}
	}
	return bits
}

// bitsToBytes — обратное к bitsFromBytes преобразование.
func bitsToBytes(bits []uint64, n int) []byte {
	out := make([]byte, (n+7)/8)
	for i := 0; i < n; i++ {
		if bits[i/64]&(1<<uint(i%64)) != 0 {
			out[i/8] |= 0x80 >> uint(i%8)
		}
	}
	return out
}
