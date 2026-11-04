//go:build ignore

// Генератор тестового JPEG: он воспроизводит фотоподобную картинку без
// сетевых загрузок. Запуск из корня модуля:
//
//	go run internal/crypto/stego/testdata/generate.go <output.jpg>
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/rand"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: generate <output.jpg>")
		os.Exit(2)
	}

	img := generate(512, 384)

	file, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		panic(err)
	}
}

func generate(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	noise := rand.New(rand.NewSource(42))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			baseR := 55 + 120*float64(x)/float64(width-1)
			baseG := 70 + 110*float64(y)/float64(height-1)
			baseB := 80 + 100*float64(x+y)/float64(width+height-2)

			baseR += disc(x, y, width*3/10, height*4/10, height/4, 45)
			baseG += disc(x, y, width*3/10, height*4/10, height/4, 35)
			baseB += disc(x, y, width*3/10, height*4/10, height/4, 20)

			baseR -= disc(x, y, width*7/10, height*6/10, height/3, 35)
			baseG -= disc(x, y, width*7/10, height*6/10, height/3, 25)
			baseB += disc(x, y, width*7/10, height*6/10, height/3, 30)

			grain := float64(noise.Intn(13) - 6)
			img.SetRGBA(x, y, color.RGBA{
				R: clamp(baseR + grain),
				G: clamp(baseG + grain),
				B: clamp(baseB + grain),
				A: 255,
			})
		}
	}

	return img
}

func disc(x, y, cx, cy, radius int, amount float64) float64 {
	dx := float64(x - cx)
	dy := float64(y - cy)
	distance := math.Sqrt(dx*dx + dy*dy)
	if distance >= float64(radius) {
		return 0
	}
	return amount * (1 - distance/float64(radius))
}

func clamp(value float64) uint8 {
	switch {
	case value < 0:
		return 0
	case value > 255:
		return 255
	default:
		return uint8(math.Round(value))
	}
}
