package hash

// Общий список хеш-функций, который использует интерфейс программы.
// Обе реализации удовлетворяют стандартному интерфейсу hash.Hash.

import "hash"

// Algorithm описывает одну доступную хеш-функцию.
type Algorithm struct {
	Name       string
	New        func() hash.Hash
	OutputBits int
	BlockSize  int
}

// Algorithms — реестр алгоритмов в порядке переключения в интерфейсе.
var Algorithms = []Algorithm{
	{
		Name:       "ГОСТ 34.11-2012 · 512",
		New:        NewGOST3411_512,
		OutputBits: 512,
		BlockSize:  gostBlockSize * 8,
	},
	{
		Name:       "ГОСТ 34.11-2012 · 256",
		New:        NewGOST3411_256,
		OutputBits: 256,
		BlockSize:  gostBlockSize * 8,
	},
	{
		Name:       "SHA-1",
		New:        NewSHA1,
		OutputBits: sha1Size * 8,
		BlockSize:  sha1BlockSize * 8,
	},
}
