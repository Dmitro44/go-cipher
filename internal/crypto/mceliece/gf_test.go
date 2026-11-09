package mceliece

import (
	"io"
	"math/rand"
	"testing"
)

// deterministicRandom даёт воспроизводимый поток байтов: тесты не зависят
// от crypto/rand и не могут «мигать».
func deterministicRandom(seed int64) io.Reader {
	return rand.New(rand.NewSource(seed))
}

func mustParams(t *testing.T, m, degree int) Params {
	t.Helper()
	params, err := NewParams(m, degree)
	if err != nil {
		t.Fatalf("NewParams(%d, %d): %v", m, degree, err)
	}
	return params
}

func mustField(t *testing.T, m int) *Field {
	t.Helper()
	field, err := NewField(m)
	if err != nil {
		t.Fatalf("NewField(%d): %v", m, err)
	}
	return field
}

func TestFieldArithmetic(t *testing.T) {
	for _, m := range []int{8, 10} {
		field := mustField(t, m)
		if got := field.Size(); got != 1<<m {
			t.Fatalf("m=%d: size = %d, want %d", m, got, 1<<m)
		}

		for _, a := range []uint16{0, 1, 2, 3, 17, uint16(field.Size() - 1)} {
			if got := field.Add(a, a); got != 0 {
				t.Errorf("m=%d: %d + %d = %d, want 0", m, a, a, got)
			}
			if got := field.Mul(a, 1); got != a {
				t.Errorf("m=%d: %d * 1 = %d, want %d", m, a, got, a)
			}
			if a == 0 {
				continue
			}
			if got := field.Mul(a, field.Inv(a)); got != 1 {
				t.Errorf("m=%d: %d * inv(%d) = %d, want 1", m, a, a, got)
			}
			root := field.Sqrt(a)
			if got := field.Mul(root, root); got != a {
				t.Errorf("m=%d: sqrt(%d)^2 = %d, want %d", m, a, got, a)
			}
			if got := field.Div(a, a); got != 1 {
				t.Errorf("m=%d: %d / %d = %d, want 1", m, a, a, got)
			}
		}

		for a := uint16(0); a < 12; a++ {
			for b := uint16(0); b < 12; b++ {
				for c := uint16(0); c < 4; c++ {
					left := field.Mul(a, field.Add(b, c))
					right := field.Add(field.Mul(a, b), field.Mul(a, c))
					if left != right {
						t.Fatalf("m=%d: distributivity fails for a=%d b=%d c=%d", m, a, b, c)
					}
				}
			}
		}
	}
}

func TestFieldRejectsUnsupportedDegree(t *testing.T) {
	if _, err := NewField(7); err == nil {
		t.Fatal("NewField(7) succeeded, want an error")
	}
}
