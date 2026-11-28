// Прямое и обратное дискретное косинусное преобразование блока 8x8.

package stego

import "math"

const blockSide = 8

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
