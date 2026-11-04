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
	"runtime"
	"strings"
	"sync"
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
	source := toRGBA(img)
	copy(stego.Pix, source.Pix)

	usedBlocks := embedBlocks(source, stego, columns, rows, bits)

	stats.UsedBlocks = usedBlocks
	stats.ChangedBlocks = usedBlocks
	stats.ChangedPixels, stats.MaxDelta, stats.MeanDelta = measureDifference(source, stego)
	stats.PSNR = psnr(source, stego)

	if err := verifyRoundTrip(stego, message); err != nil {
		return nil, stats, err
	}

	return stego, stats, nil
}

// embedBlocks раскладывает биты по блокам слева направо и сверху вниз. Блоки
// друг от друга не зависят, поэтому строки блоков делятся на полосы, и каждая
// полоса обрабатывается своей горутиной: номер блока задаёт его биты, так что
// синхронизация не нужна. На 4K это сокращает время с секунд до десятых.
func embedBlocks(source, stego *image.RGBA, columns, rows int, bits []byte) int {
	workers := blockWorkers(rows)
	used := make([]int, workers)

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		start := rows * worker / workers
		end := rows * (worker + 1) / workers

		wg.Add(1)
		go func(slot, start, end int) {
			defer wg.Done()
			used[slot] = embedBand(source, stego, columns, start, end, bits)
		}(worker, start, end)
	}
	wg.Wait()

	total := 0
	for _, count := range used {
		total += count
	}
	return total
}

func embedBand(source, stego *image.RGBA, columns, startRow, endRow int, bits []byte) int {
	used := 0
	for blockY := startRow; blockY < endRow; blockY++ {
		for blockX := 0; blockX < columns; blockX++ {
			bitIndex := 2 * (blockY*columns + blockX)
			if bitIndex >= len(bits) {
				return used
			}

			x0, y0 := blockX*blockSide, blockY*blockSide
			original := readLumaBlock(source, x0, y0)
			coefficients := forwardDCT(original)

			for i, position := range payloadCoefficients {
				if bitIndex+i >= len(bits) {
					break
				}
				coefficients[position.u][position.v] = embedBit(
					coefficients[position.u][position.v], bits[bitIndex+i],
				)
			}

			rebuilt := inverseDCT(coefficients)
			writeBlockDelta(stego, source, x0, y0, original, rebuilt)
			used++
		}
	}
	return used
}

// Extract читает сообщение из коэффициентов изображения.
func Extract(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, ErrNilImage
	}

	source := toRGBA(img)
	columns, rows := source.Bounds().Dx()/blockSide, source.Bounds().Dy()/blockSide
	capacity := 2 * columns * rows
	if capacity < headerSize*8 {
		return nil, ErrNoPayload
	}

	bits := make([]byte, capacity)
	extractBits(source, columns, rows, bits)

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
		if errors.Is(err, ErrNoPayload) {
			return fmt.Errorf(
				"%w: areas of pure black or white clip the brightness change; try a picture with more headroom",
				ErrSelfCheck,
			)
		}
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

// quote: helpers below work on *image.RGBA.

// toRGBA приводит изображение к RGBA: дальше пиксели читаются и пишутся по
// слайсу Pix, без интерфейсных вызовов At/Set на каждый пиксель.
func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	bounds := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(out, out.Bounds(), img, bounds.Min, draw.Src)
	return out
}

// blockWorkers ограничивает число горутин числом полос и ядер.
func blockWorkers(rows int) int {
	workers := runtime.NumCPU()
	if workers > rows {
		workers = rows
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

// extractBits читает биты из коэффициентов блоков — та же разбивка на полосы,
// что и при встраивании, но только чтение.
func extractBits(source *image.RGBA, columns, rows int, bits []byte) {
	workers := blockWorkers(rows)

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		start := rows * worker / workers
		end := rows * (worker + 1) / workers

		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for blockY := start; blockY < end; blockY++ {
				for blockX := 0; blockX < columns; blockX++ {
					coefficients := forwardDCT(readLumaBlock(source, blockX*blockSide, blockY*blockSide))
					index := 2 * (blockY*columns + blockX)
					for i, position := range payloadCoefficients {
						bits[index+i] = readBit(coefficients[position.u][position.v])
					}
				}
			}
		}(start, end)
	}
	wg.Wait()
}

// readLumaBlock читает блок 8×8 и переводит пиксели в яркость.
func readLumaBlock(img *image.RGBA, x0, y0 int) [blockSide][blockSide]float64 {
	var block [blockSide][blockSide]float64
	for y := 0; y < blockSide; y++ {
		offset := img.PixOffset(x0, y0+y)
		for x := 0; x < blockSide; x++ {
			p := offset + 4*x
			block[y][x] = lumaRGB(img.Pix[p], img.Pix[p+1], img.Pix[p+2])
		}
	}
	return block
}

// writeBlockDelta добавляет к R, G и B одну и ту же разность яркостей.
// Прибавка ко всем каналам сохраняет оттенок, а яркость результата
// совпадает с восстановленной яркостью блока с точностью до округления.
func writeBlockDelta(stego, source *image.RGBA, x0, y0 int, original, rebuilt [blockSide][blockSide]float64) {
	for y := 0; y < blockSide; y++ {
		offset := source.PixOffset(x0, y0+y)
		for x := 0; x < blockSide; x++ {
			p := offset + 4*x
			delta := rebuilt[y][x] - original[y][x]
			stego.Pix[p] = clampChannel(float64(source.Pix[p]) + delta)
			stego.Pix[p+1] = clampChannel(float64(source.Pix[p+1]) + delta)
			stego.Pix[p+2] = clampChannel(float64(source.Pix[p+2]) + delta)
			stego.Pix[p+3] = source.Pix[p+3]
		}
	}
}

func lumaRGB(r, g, b uint8) float64 {
	return 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
}

// Luma переводит пиксель в яркость по формуле JPEG. Одинаковая формула
// используется и при встраивании, и при извлечении.
func Luma(pixel color.Color) float64 {
	value := color.NRGBAModel.Convert(pixel).(color.NRGBA)
	return lumaRGB(value.R, value.G, value.B)
}

func lumaRGBFrom(pixel color.Color) float64 {
	value := color.NRGBAModel.Convert(pixel).(color.NRGBA)
	return lumaRGB(value.R, value.G, value.B)
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
