package hash

// Проверка SHA-1 по FIPS 180-4 и сверка со стандартной библиотекой.

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"math/rand"
	"strings"
	"testing"
)

func TestSHA1KnownAnswers(t *testing.T) {
	million := strings.Repeat("a", 1_000_000)

	cases := []struct {
		name    string
		message string
		want    string
	}{
		{"пустое сообщение", "", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"abc", "abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{"448 бит", "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq", "84983e441c3bd26ebaae4aa1f95129e5e54670f1"},
		{"миллион a", million, "34aa973cd4c4daa4f61eeb2bdbad27316534016f"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewSHA1()
			h.Write([]byte(tc.message))
			if got := hex.EncodeToString(h.Sum(nil)); got != tc.want {
				t.Fatalf("получено %s\nожидалось  %s", got, tc.want)
			}
		})
	}
}

func TestSHA1Incremental(t *testing.T) {
	message := make([]byte, 513)
	rng := rand.New(rand.NewSource(5))
	rng.Read(message)

	oneShot := NewSHA1()
	oneShot.Write(message)
	want := hex.EncodeToString(oneShot.Sum(nil))

	for _, chunk := range []int{1, 2, 13, 55, 56, 63, 64, 65} {
		h := NewSHA1()
		for start := 0; start < len(message); start += chunk {
			end := start + chunk
			if end > len(message) {
				end = len(message)
			}
			h.Write(message[start:end])
		}
		if got := hex.EncodeToString(h.Sum(nil)); got != want {
			t.Fatalf("размер порции %d: получено %s, ожидалось %s", chunk, got, want)
		}
	}
}

// Сверка с эталонной реализацией стандартной библиотеки на детерминированных
// псевдослучайных данных разной длины.
func TestSHA1MatchesStandardLibrary(t *testing.T) {
	rng := rand.New(rand.NewSource(2026))

	for length := 0; length <= 700; length++ {
		message := make([]byte, length)
		rng.Read(message)

		got := NewSHA1()
		got.Write(message)

		want := sha1.Sum(message)
		if !bytes.Equal(got.Sum(nil), want[:]) {
			t.Fatalf("длина %d: получено %x, ожидалось %x", length, got.Sum(nil), want)
		}
	}
}
