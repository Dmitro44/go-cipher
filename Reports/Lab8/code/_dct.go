package stego

import "math"

// blockSide — сторона блока JPEG. В стандарте изображение делится на блоки
// 8×8, и дискретное косинусное преобразование применяется к каждому блоку.
const blockSide = 8

// dctBasis[u][x] = C(u) * cos((2x+1)uπ/16), где C(0) = 1/√2, C(u>0) = 1.
// Таблица считается один раз: базисные функции от блока не зависят.
var dctBasis = newDCTBasis()

func newDCTBasis() [blockSide][blockSide]float64 {
	var basis [blockSide][blockSide]float64

	for u := 0; u < blockSide; u++ {
		scale := 1.0
		if u == 0 {
			scale = 1 / math.Sqrt2
		}
		for x := 0; x < blockSide; x++ {
			basis[u][x] = scale * math.Cos(
				float64(2*x+1)*float64(u)*math.Pi/float64(2*blockSide),
			)
		}
	}

	return basis
}

// forwardDCT вычисляет двумерное DCT-II блока 8×8.
//
//	F(u,v) = 1/4 · C(u)C(v) · Σx Σy f(x,y) cos((2x+1)uπ/16) cos((2y+1)vπ/16)
//
// Преобразование разделимо, поэтому сначала считаются суммы по строкам
// (горизонтальная частота u), затем по столбцам (вертикальная частота v).
func forwardDCT(block [blockSide][blockSide]float64) [blockSide][blockSide]float64 {
	var rows [blockSide][blockSide]float64

	for y := 0; y < blockSide; y++ {
		for u := 0; u < blockSide; u++ {
			var sum float64
			for x := 0; x < blockSide; x++ {
				sum += block[y][x] * dctBasis[u][x]
			}
			rows[y][u] = sum
		}
	}

	var coefficients [blockSide][blockSide]float64

	for v := 0; v < blockSide; v++ {
		for u := 0; u < blockSide; u++ {
			var sum float64
			for y := 0; y < blockSide; y++ {
				sum += rows[y][u] * dctBasis[v][y]
			}
			coefficients[u][v] = sum / 4
		}
	}

	return coefficients
}

// inverseDCT восстанавливает блок 8×8 из его спектра:
//
//	f(x,y) = 1/4 · Σu Σv C(u)C(v) F(u,v) cos((2x+1)uπ/16) cos((2y+1)vπ/16)
//
// Как и прямое преобразование, обратное выполняется разделимо.
func inverseDCT(coefficients [blockSide][blockSide]float64) [blockSide][blockSide]float64 {
	var columns [blockSide][blockSide]float64

	for v := 0; v < blockSide; v++ {
		for x := 0; x < blockSide; x++ {
			var sum float64
			for u := 0; u < blockSide; u++ {
				sum += coefficients[u][v] * dctBasis[u][x]
			}
			columns[v][x] = sum
		}
	}

	var block [blockSide][blockSide]float64

	for y := 0; y < blockSide; y++ {
		for x := 0; x < blockSide; x++ {
			var sum float64
			for v := 0; v < blockSide; v++ {
				sum += columns[v][x] * dctBasis[v][y]
			}
			block[y][x] = sum / 4
		}
	}

	return block
}
