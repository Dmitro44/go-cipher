package stego

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// quantizationStep — шаг квантования QIM. Больше шаг — устойчивее бит к
// повторному сжатию и шуму, но заметнее искажение блока. При q = 16
// изменение среднего коэффициента не превышает 8, что на глаз незаметно.
const quantizationStep = 16.0

const (
	formatVersion = 1
	headerSize    = len(magic) + 1 + 4
)

// magic отделяет наше сообщение от произвольных битов, которые можно
// прочитать из коэффициентов обычной фотографии.
var magic = [7]byte{'M', 'Z', 'I', 'S', 'T', 'E', 'G'}

var (
	ErrNilImage         = errors.New("stego: nil image")
	ErrPayloadTooLarge  = errors.New("stego: payload does not fit into image")
	ErrNoPayload        = errors.New("stego: no MZISTEG payload found")
	ErrBadVersion       = errors.New("stego: unsupported payload version")
	ErrCorruptPayload   = errors.New("stego: payload length is inconsistent with capacity")
	ErrSelfCheck        = errors.New("stego: embedded payload does not survive PNG round trip")
	ErrMessageTooLong   = errors.New("stego: message is too long")
	ErrUnsupportedInput = errors.New("stego: unsupported image format")
)

// coefficient задаёт позицию среднечастотного коэффициента DCT. Низкие
// частоты слишком заметны для глаза, высокие — неустойчивы к сжатию,
// поэтому выбраны два симметричных среднечастотных коэффициента.
type coefficient struct {
	u, v int
}

var payloadCoefficients = [2]coefficient{
	{u: 3, v: 4},
	{u: 4, v: 3},
}

// Stats описывает результат встраивания и заполняется так, чтобы интерфейс
// мог показать цену сокрытия: ёмкость, искажение и число изменённых пикселей.
type Stats struct {
	Width         int
	Height        int
	CapacityBits  int
	UsedBits      int
	Blocks        int
	UsedBlocks    int
	ChangedBlocks int
	ChangedPixels int
	MaxDelta      float64
	MeanDelta     float64
	PSNR          float64
}

// Capacity возвращает ёмкость изображения в битах: по два бита на каждый
// полный блок 8×8. Неполные блоки по краям не используются.
func Capacity(img image.Image) int {
	if img == nil {
		return 0
	}
	bounds := img.Bounds()
	return 2 * (bounds.Dx() / blockSide) * (bounds.Dy() / blockSide)
}

// DecodeFile читает изображение из файла. Поддерживаются JPEG и PNG:
// JPEG — исходный контейнер, PNG — контейнер, в который мы записываем
// результат без повторного квантования.
func DecodeFile(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if format != "jpeg" && format != "png" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedInput, format)
	}
	return img, nil
}

// WritePNG сохраняет контейнер без потерь. Повторное сохранение в JPEG
// недопустимо: квантование JPEG изменит коэффициенты и разрушит биты.
func WritePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	if err := png.Encode(file, img); err != nil {
		file.Close()
		return fmt.Errorf("encode container: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close container: %w", err)
	}
	return nil
}

// OutputPath строит имя контейнера рядом с исходным файлом:
// photo.jpg → stego-photo.png.
func OutputPath(sourcePath string) string {
	dir := filepath.Dir(sourcePath)
	name := filepath.Base(sourcePath)
	ext := filepath.Ext(name)
	return filepath.Join(dir, "stego-"+strings.TrimSuffix(name, ext)+".png")
}

// Embed скрывает сообщение в среднечастотных коэффициентах DCT яркости.
// Возвращается изображение RGBA, которое нужно сохранять только как PNG.
func Embed(img image.Image, message []byte) (*image.RGBA, Stats, error) {
	if img == nil {
		return nil, Stats{}, ErrNilImage
	}
	if uint64(len(message)) > math.MaxUint32 {
		return nil, Stats{}, ErrMessageTooLong
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	columns, rows := width/blockSide, height/blockSide
	capacity := 2 * columns * rows

	payload := frame(message)
	bits := bytesToBits(payload)

	stats := Stats{
		Width:        width,
		Height:       height,
		CapacityBits: capacity,
		UsedBits:     len(bits),
		Blocks:       columns * rows,
	}
	if len(bits) > capacity {
		return nil, stats, fmt.Errorf(
			"%w: need %d bits, capacity is %d bits",
			ErrPayloadTooLarge, len(bits), capacity,
		)
	}

	stego := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(stego, stego.Bounds(), img, bounds.Min, draw.Src)

	bitIndex := 0
	usedBlocks := 0

	for blockY := 0; blockY < rows; blockY++ {
		for blockX := 0; blockX < columns; blockX++ {
			x0 := bounds.Min.X + blockX*blockSide
			y0 := bounds.Min.Y + blockY*blockSide

			original := readLumaBlock(img, x0, y0)
			coefficients := forwardDCT(original)

			usedInBlock := 0
			for _, position := range payloadCoefficients {
				if bitIndex >= len(bits) {
					break
				}
				coefficients[position.u][position.v] = embedBit(
					coefficients[position.u][position.v],
					bits[bitIndex],
				)
				bitIndex++
				usedInBlock++
			}
			if usedInBlock == 0 {
				continue
			}
			usedBlocks++

			rebuilt := inverseDCT(coefficients)
			writeBlockWithDelta(stego, img, x0, y0, original, rebuilt)
		}
	}

	stats.UsedBlocks = usedBlocks
	stats.ChangedBlocks = usedBlocks
	stats.ChangedPixels, stats.MaxDelta, stats.MeanDelta = measureDifference(img, stego)
	stats.PSNR = psnr(img, stego)

	if err := verifyRoundTrip(stego, message); err != nil {
		return nil, stats, err
	}

	return stego, stats, nil
}

// Extract читает сообщение из коэффициентов изображения.
func Extract(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, ErrNilImage
	}

	bounds := img.Bounds()
	columns, rows := bounds.Dx()/blockSide, bounds.Dy()/blockSide
	capacity := 2 * columns * rows
	if capacity < headerSize*8 {
		return nil, ErrNoPayload
	}

	bits := make([]byte, 0, capacity)
	for blockY := 0; blockY < rows; blockY++ {
		for blockX := 0; blockX < columns; blockX++ {
			x0 := bounds.Min.X + blockX*blockSide
			y0 := bounds.Min.Y + blockY*blockSide

			coefficients := forwardDCT(readLumaBlock(img, x0, y0))
			for _, position := range payloadCoefficients {
				bits = append(bits, readBit(coefficients[position.u][position.v]))
			}
		}
	}

	if !bytes.Equal(bitsToBytes(bits[:len(magic)*8]), magic[:]) {
		return nil, ErrNoPayload
	}

	version := bitsToBytes(bits[len(magic)*8 : len(magic)*8+8])[0]
	if version != formatVersion {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrBadVersion, version, formatVersion)
	}

	lengthStart := (len(magic) + 1) * 8
	lengthBits := bits[lengthStart : lengthStart+32]
	length := int(binary.BigEndian.Uint32(bitsToBytes(lengthBits)))

	maxPayload := capacity/8 - headerSize
	if length > maxPayload {
		return nil, fmt.Errorf(
			"%w: declared %d bytes, maximum is %d",
			ErrCorruptPayload, length, maxPayload,
		)
	}

	end := (headerSize + length) * 8
	return bitsToBytes(bits[headerSize*8 : end]), nil
}

// embedBit ставит коэффициент в центр квантованной ячейки, номер которой
// имеет нужную чётность. Решающая граница находится на расстоянии q/2 от
// центра, поэтому ошибка округления при обратном DCT и сохранении в PNG
// не может перевернуть бит.
func embedBit(value float64, bit byte) float64 {
	index := int(math.Round(value / quantizationStep))
	if parity(index) != bit {
		lower := float64(index-1) * quantizationStep
		upper := float64(index+1) * quantizationStep
		if math.Abs(value-lower) < math.Abs(value-upper) {
			index--
		} else {
			index++
		}
	}
	return float64(index) * quantizationStep
}

// readBit повторяет квантование и возвращает чётность номера ячейки.
func readBit(value float64) byte {
	return parity(int(math.Round(value / quantizationStep)))
}

func parity(index int) byte {
	return byte(((index % 2) + 2) % 2)
}

func verifyRoundTrip(stego *image.RGBA, message []byte) error {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, stego); err != nil {
		return fmt.Errorf("%w: encode PNG: %v", ErrSelfCheck, err)
	}

	decoded, err := png.Decode(&encoded)
	if err != nil {
		return fmt.Errorf("%w: decode PNG: %v", ErrSelfCheck, err)
	}

	extracted, err := Extract(decoded)
	if err != nil {
		return fmt.Errorf("%w: extraction after saving failed: %w", ErrSelfCheck, err)
	}
	if !bytes.Equal(extracted, message) {
		return fmt.Errorf("%w: payload changed after PNG round trip", ErrSelfCheck)
	}
	return nil
}

func frame(message []byte) []byte {
	payload := make([]byte, headerSize+len(message))
	copy(payload, magic[:])
	payload[len(magic)] = formatVersion
	binary.BigEndian.PutUint32(payload[len(magic)+1:], uint32(len(message)))
	copy(payload[headerSize:], message)
	return payload
}

func bytesToBits(data []byte) []byte {
	bits := make([]byte, 0, len(data)*8)
	for _, value := range data {
		for shift := 7; shift >= 0; shift-- {
			bits = append(bits, (value>>uint(shift))&1)
		}
	}
	return bits
}

func bitsToBytes(bits []byte) []byte {
	out := make([]byte, len(bits)/8)
	for i := range out {
		for bit := 0; bit < 8; bit++ {
			out[i] = out[i]<<1 | bits[i*8+bit]
		}
	}
	return out
}

// Luma переводит пиксель в яркость по формуле JPEG. Одинаковая формула
// используется и при встраивании, и при извлечении.
func Luma(pixel color.Color) float64 {
	value := color.NRGBAModel.Convert(pixel).(color.NRGBA)
	return 0.299*float64(value.R) + 0.587*float64(value.G) + 0.114*float64(value.B)
}

func readLumaBlock(img image.Image, x0, y0 int) [blockSide][blockSide]float64 {
	var block [blockSide][blockSide]float64
	for y := 0; y < blockSide; y++ {
		for x := 0; x < blockSide; x++ {
			block[y][x] = Luma(img.At(x0+x, y0+y))
		}
	}
	return block
}

// writeBlockWithDelta добавляет к R, G и B одну и ту же разность яркостей.
// Прибавка ко всем каналам сохраняет оттенок, а яркость результата
// совпадает с восстановленной яркостью блока с точностью до округления.
func writeBlockWithDelta(stego *image.RGBA, source image.Image, x0, y0 int, original, rebuilt [blockSide][blockSide]float64) {
	for y := 0; y < blockSide; y++ {
		for x := 0; x < blockSide; x++ {
			delta := rebuilt[y][x] - original[y][x]
			pixel := color.NRGBAModel.Convert(source.At(x0+x, y0+y)).(color.NRGBA)
			stego.SetRGBA(x0+x, y0+y, color.RGBA{
				R: clampChannel(float64(pixel.R) + delta),
				G: clampChannel(float64(pixel.G) + delta),
				B: clampChannel(float64(pixel.B) + delta),
				A: pixel.A,
			})
		}
	}
}

func clampChannel(value float64) uint8 {
	rounded := math.Round(value)
	if rounded < 0 {
		return 0
	}
	if rounded > 255 {
		return 255
	}
	return uint8(rounded)
}

func measureDifference(original, stego image.Image) (changedPixels int, maxDelta, meanDelta float64) {
	bounds := original.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0, 0, 0
	}

	var sum float64
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			before := original.At(bounds.Min.X+x, bounds.Min.Y+y)
			after := stego.At(bounds.Min.X+x, bounds.Min.Y+y)
			if color.NRGBAModel.Convert(before) != color.NRGBAModel.Convert(after) {
				changedPixels++
			}
			delta := math.Abs(Luma(after) - Luma(before))
			sum += delta
			if delta > maxDelta {
				maxDelta = delta
			}
		}
	}

	return changedPixels, maxDelta, sum / float64(total)
}

func psnr(original, stego image.Image) float64 {
	bounds := original.Bounds()
	var squaredError float64

	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			before := color.NRGBAModel.Convert(original.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			after := color.NRGBAModel.Convert(stego.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)

			dr := float64(before.R) - float64(after.R)
			dg := float64(before.G) - float64(after.G)
			db := float64(before.B) - float64(after.B)
			squaredError += dr*dr + dg*dg + db*db
		}
	}

	samples := float64(bounds.Dx() * bounds.Dy() * 3)
	if samples == 0 {
		return 0
	}
	mse := squaredError / samples
	if mse == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(255*255/mse)
}
