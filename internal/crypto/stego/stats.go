package stego

import (
	"image"
	"math"
	"sync"
)

// pairStats — итог обхода одной полосы строк пары изображений.
type pairStats struct {
	changedPixels int
	maxDelta      float64
	sumDelta      float64
	squaredError  float64
	samples       int
}

// compare обходит оба изображения один раз, полосами строк, и возвращает
// частичные результаты каждой полосы в её порядке.
func compare(original, stego *image.RGBA) []pairStats {
	bounds := original.Bounds()
	height := bounds.Dy()
	workers := blockWorkers(height)
	out := make([]pairStats, workers)

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		start := height * worker / workers
		end := height * (worker + 1) / workers

		wg.Add(1)
		go func(slot, start, end int) {
			defer wg.Done()
			out[slot] = compareBand(original, stego, start, end)
		}(worker, start, end)
	}
	wg.Wait()

	return out
}

func compareBand(original, stego *image.RGBA, startRow, endRow int) pairStats {
	var stats pairStats
	width := original.Bounds().Dx()

	for y := startRow; y < endRow; y++ {
		row := original.PixOffset(0, y)
		for x := 0; x < width; x++ {
			p := row + 4*x
			before := lumaRGB(original.Pix[p], original.Pix[p+1], original.Pix[p+2])
			after := lumaRGB(stego.Pix[p], stego.Pix[p+1], stego.Pix[p+2])

			if original.Pix[p] != stego.Pix[p] ||
				original.Pix[p+1] != stego.Pix[p+1] ||
				original.Pix[p+2] != stego.Pix[p+2] {
				stats.changedPixels++
			}

			delta := math.Abs(after - before)
			stats.sumDelta += delta
			if delta > stats.maxDelta {
				stats.maxDelta = delta
			}

			dr := float64(original.Pix[p]) - float64(stego.Pix[p])
			dg := float64(original.Pix[p+1]) - float64(stego.Pix[p+1])
			db := float64(original.Pix[p+2]) - float64(stego.Pix[p+2])
			stats.squaredError += dr*dr + dg*dg + db*db
			stats.samples += 3
		}
	}

	return stats
}

// measureDifference сводит полосы в три числа: изменённые пиксели,
// максимальную и среднюю разность яркости.
func measureDifference(original, stego *image.RGBA) (changedPixels int, maxDelta, meanDelta float64) {
	bands := compare(original, stego)

	var sum float64
	for _, band := range bands {
		changedPixels += band.changedPixels
		sum += band.sumDelta
		if band.maxDelta > maxDelta {
			maxDelta = band.maxDelta
		}
	}

	total := original.Bounds().Dx() * original.Bounds().Dy()
	if total == 0 {
		return 0, 0, 0
	}
	return changedPixels, maxDelta, sum / float64(total)
}

// psnr считает пиковое отношение сигнал/шум по всем трём каналам.
func psnr(original, stego *image.RGBA) float64 {
	var squaredError float64
	samples := 0
	for _, band := range compare(original, stego) {
		squaredError += band.squaredError
		samples += band.samples
	}

	if samples == 0 {
		return 0
	}
	mse := squaredError / float64(samples)
	if mse == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(255*255/mse)
}
