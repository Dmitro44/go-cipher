package stego

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"os"
	"testing"
)

func loadSample(t *testing.T) image.Image {
	t.Helper()

	img, err := DecodeFile("testdata/sample.jpg")
	if err != nil {
		t.Fatalf("decode sample: %v", err)
	}
	return img
}

func TestDCTRoundTrip(t *testing.T) {
	random := rand.New(rand.NewSource(1))

	var block [blockSide][blockSide]float64
	for y := 0; y < blockSide; y++ {
		for x := 0; x < blockSide; x++ {
			block[y][x] = random.Float64() * 255
		}
	}

	rebuilt := inverseDCT(forwardDCT(block))

	maxError := 0.0
	for y := 0; y < blockSide; y++ {
		for x := 0; x < blockSide; x++ {
			maxError = math.Max(maxError, math.Abs(block[y][x]-rebuilt[y][x]))
		}
	}

	if maxError >= 1e-9 {
		t.Fatalf("DCT round trip error = %g, want < 1e-9", maxError)
	}
}

func TestEmbedExtractRoundTrip(t *testing.T) {
	img := loadSample(t)
	capacityBytes := Capacity(img)/8 - headerSize

	tests := []struct {
		name    string
		message []byte
	}{
		{name: "empty", message: []byte{}},
		{name: "short", message: []byte("лабораторная работа № 8")},
		{name: "binary", message: []byte{0x00, 0x01, 0xFF, 0x00, 0x80}},
		{name: "near capacity", message: bytes.Repeat([]byte{0xA5}, capacityBytes-1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stego, stats, err := Embed(img, test.message)
			if err != nil {
				t.Fatalf("embed: %v", err)
			}

			extracted, err := Extract(stego)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if !bytes.Equal(extracted, test.message) {
				t.Fatalf("extracted %d bytes, want %d", len(extracted), len(test.message))
			}

			if stats.ChangedPixels == 0 {
				t.Fatal("no pixels changed")
			}
			if stats.PSNR <= 30 {
				t.Fatalf("PSNR = %.2f dB, want > 30 dB", stats.PSNR)
			}
		})
	}
}

func TestContainerPNGRoundTrip(t *testing.T) {
	img := loadSample(t)
	message := bytes.Repeat([]byte("PNG "), 40)

	stego, _, err := Embed(img, message)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, stego); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}

	decoded, err := png.Decode(&encoded)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}

	extracted, err := Extract(decoded)
	if err != nil {
		t.Fatalf("extract from PNG: %v", err)
	}
	if !bytes.Equal(extracted, message) {
		t.Fatalf("payload changed after PNG round trip")
	}
}

func TestExtractCleanImageRejected(t *testing.T) {
	img := loadSample(t)

	if _, err := Extract(img); !errors.Is(err, ErrNoPayload) {
		t.Fatalf("extract clean image error = %v, want ErrNoPayload", err)
	}
}

func TestPayloadTooLarge(t *testing.T) {
	img := loadSample(t)
	tooLarge := bytes.Repeat([]byte{0x5A}, Capacity(img)/8-headerSize+1)

	_, stats, err := Embed(img, tooLarge)
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("embed over capacity error = %v, want ErrPayloadTooLarge", err)
	}
	if stats.CapacityBits != Capacity(img) {
		t.Fatalf("stats capacity = %d, want %d", stats.CapacityBits, Capacity(img))
	}
}

func TestCapacityBoundary(t *testing.T) {
	img := loadSample(t)
	maxPayload := Capacity(img)/8 - headerSize

	if _, _, err := Embed(img, bytes.Repeat([]byte{1}, maxPayload)); err != nil {
		t.Fatalf("embed exactly at capacity: %v", err)
	}
	if _, _, err := Embed(img, bytes.Repeat([]byte{1}, maxPayload+1)); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("embed one byte over capacity error = %v, want ErrPayloadTooLarge", err)
	}
}

func TestPSNRForOneKilobytePayload(t *testing.T) {
	img := makePhoto(768, 512)
	message := bytes.Repeat([]byte("stego"), 1024/5)

	stego, stats, err := Embed(img, message)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	extracted, err := Extract(stego)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !bytes.Equal(extracted, message) {
		t.Fatal("1 KB payload did not round trip")
	}

	t.Logf(
		"capacity=%d B used=%d B PSNR=%.2f dB changed=%d px max=%.2f mean=%.3f",
		stats.CapacityBits/8, stats.UsedBits/8, stats.PSNR,
		stats.ChangedPixels, stats.MaxDelta, stats.MeanDelta,
	)

	if stats.PSNR <= 30 {
		t.Fatalf("PSNR = %.2f dB, want > 30 dB", stats.PSNR)
	}
}

func TestEmbedIsDeterministic(t *testing.T) {
	img := loadSample(t)
	message := []byte("deterministic container")

	first, firstStats, err := Embed(img, message)
	if err != nil {
		t.Fatalf("first embed: %v", err)
	}
	second, secondStats, err := Embed(img, message)
	if err != nil {
		t.Fatalf("second embed: %v", err)
	}

	if !bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("same input produced different pixels")
	}
	if firstStats != secondStats {
		t.Fatalf("stats differ: %+v vs %+v", firstStats, secondStats)
	}
}

func makePhoto(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	noise := rand.New(rand.NewSource(7))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := 60 + 110*float64(x)/float64(width)
			g := 70 + 100*float64(y)/float64(height)
			b := 90 + 90*float64(x+y)/float64(width+height)
			grain := float64(noise.Intn(11) - 5)

			img.SetRGBA(x, y, color.RGBA{
				R: clampChannel(r + grain),
				G: clampChannel(g + grain),
				B: clampChannel(b + grain),
				A: 255,
			})
		}
	}
	return img
}

func TestDecodeFileRejectsGarbage(t *testing.T) {
	path := t.TempDir() + "/not-an-image.jpg"
	if err := os.WriteFile(path, []byte("definitely not JPEG"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeFile(path); err == nil {
		t.Fatal("DecodeFile accepted garbage")
	}
}

func TestOutputPath(t *testing.T) {
	got := OutputPath("/tmp/photos/meadow.jpeg")
	want := "/tmp/photos/stego-meadow.png"
	if got != want {
		t.Fatalf("OutputPath = %q, want %q", got, want)
	}
}

func TestEmbedNilImage(t *testing.T) {
	if _, _, err := Embed(nil, []byte("x")); !errors.Is(err, ErrNilImage) {
		t.Fatalf("embed nil error = %v, want ErrNilImage", err)
	}
}

func TestJPEGReEncodingAtLowQualityDestroysPayload(t *testing.T) {
	img := loadSample(t)
	message := []byte("survives PNG only")

	stego, _, err := Embed(img, message)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, stego, &jpeg.Options{Quality: 70}); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	recompressed, err := jpeg.Decode(&jpegBytes)
	if err != nil {
		t.Fatalf("decode JPEG: %v", err)
	}

	extracted, err := Extract(recompressed)
	if err == nil && bytes.Equal(extracted, message) {
		t.Fatal("payload survived JPEG re-encoding; the report expects it to be destroyed")
	}
}
