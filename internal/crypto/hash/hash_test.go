package hash

// Свойства всех зарегистрированных хеш-функций: лавинный эффект и параметры.

import (
	"bytes"
	"encoding/hex"
	"math/bits"
	"testing"
)

func digestOf(alg Algorithm, message []byte) []byte {
	h := alg.New()
	h.Write(message)
	return h.Sum(nil)
}

func TestAlgorithmRegistry(t *testing.T) {
	want := []struct {
		name  string
		bytes int
	}{
		{"ГОСТ 34.11-2012 · 512", 64},
		{"ГОСТ 34.11-2012 · 256", 32},
		{"SHA-1", 20},
	}

	if len(Algorithms) != len(want) {
		t.Fatalf("в реестре %d алгоритмов вместо %d", len(Algorithms), len(want))
	}

	for i, alg := range Algorithms {
		if alg.Name != want[i].name {
			t.Errorf("алгоритм %d: имя %q, ожидалось %q", i, alg.Name, want[i].name)
		}
		if alg.OutputBits != want[i].bytes*8 {
			t.Errorf("%s: длина выхода %d бит, ожидалось %d", alg.Name, alg.OutputBits, want[i].bytes*8)
		}
		if got := len(digestOf(alg, []byte("abc"))); got != want[i].bytes {
			t.Errorf("%s: дайджест %d байт, ожидалось %d", alg.Name, got, want[i].bytes)
		}
		if alg.New().BlockSize() != alg.BlockSize/8 {
			t.Errorf("%s: размер блока не совпадает с реестром", alg.Name)
		}
	}
}

func TestAvalanche(t *testing.T) {
	message := []byte("MZI lab 5: avalanche property test, fixed message of 64 bytes!")

	for _, alg := range Algorithms {
		t.Run(alg.Name, func(t *testing.T) {
			base := digestOf(alg, message)
			outputBits := len(base) * 8

			minBits, maxBits, total := outputBits, 0, 0
			for bit := 0; bit < len(message)*8; bit++ {
				flipped := append([]byte(nil), message...)
				flipped[bit/8] ^= 1 << uint(bit%8)

				changed := 0
				for i, b := range digestOf(alg, flipped) {
					changed += bits.OnesCount8(b ^ base[i])
				}

				if changed < minBits {
					minBits = changed
				}
				if changed > maxBits {
					maxBits = changed
				}
				total += changed

				if percent := 100 * float64(changed) / float64(outputBits); percent < 30 || percent > 70 {
					t.Fatalf("бит входа %d: изменилось %d из %d бит (%.1f%%)", bit, changed, outputBits, percent)
				}
			}

			t.Logf("%s: минимум %d, максимум %d, в среднем %.1f из %d бит (%.1f%%)",
				alg.Name, minBits, maxBits,
				float64(total)/float64(len(message)*8), outputBits,
				100*float64(total)/float64(len(message)*8)/float64(outputBits))
		})
	}
}

func TestSingleBitChangeChangesDigest(t *testing.T) {
	a := []byte("ГОСТ 34.11-2012")
	b := []byte("ГОСТ 34.11-2013")

	for _, alg := range Algorithms {
		if hex.EncodeToString(digestOf(alg, a)) == hex.EncodeToString(digestOf(alg, b)) {
			t.Errorf("%s: разные сообщения дали одинаковый дайджест", alg.Name)
		}
	}
}

func BenchmarkAlgorithms(b *testing.B) {
	message := bytes.Repeat([]byte("MZI hash benchmark "), 64*1024)

	for _, alg := range Algorithms {
		b.Run(alg.Name, func(b *testing.B) {
			b.SetBytes(int64(len(message)))
			for i := 0; i < b.N; i++ {
				h := alg.New()
				h.Write(message)
				h.Sum(nil)
			}
		})
	}
}
