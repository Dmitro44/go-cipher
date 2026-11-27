package mceliece

import (
	"errors"
	"testing"
)

func TestPattersonCorrectsWeightT(t *testing.T) {
	for _, preset := range [][2]int{{8, 8}, {10, 38}} {
		params := mustParams(t, preset[0], preset[1])
		code, err := NewGoppaCode(params, deterministicRandom(21))
		if err != nil {
			t.Fatalf("NewGoppaCode(%v): %v", preset, err)
		}

		message := newVector(params.K)
		for bit := 0; bit < params.K; bit += 3 {
			setVectorBit(message, bit)
		}
		codeword := code.Matrix.MulVector(message)

		errorsVector, err := randomErrorVector(params.N, params.T, deterministicRandom(22))
		if err != nil {
			t.Fatalf("randomErrorVector: %v", err)
		}
		received := make([]uint64, len(codeword))
		for i := range codeword {
			received[i] = codeword[i] ^ errorsVector[i]
		}

		corrected, err := code.Decoder().Decode(received)
		if err != nil {
			t.Fatalf("params %v: Decode: %v", preset, err)
		}
		for i := range codeword {
			if corrected[i] != codeword[i] {
				t.Fatalf("params %v: decoded word differs from codeword", preset)
			}
		}
	}
}

func TestPattersonNoErrors(t *testing.T) {
	params := mustParams(t, 8, 8)
	code, err := NewGoppaCode(params, deterministicRandom(23))
	if err != nil {
		t.Fatalf("NewGoppaCode: %v", err)
	}
	message := newVector(params.K)
	setVectorBit(message, 7)
	codeword := code.Matrix.MulVector(message)

	corrected, err := code.Decoder().Decode(codeword)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for i := range codeword {
		if corrected[i] != codeword[i] {
			t.Fatal("decoder changed an error-free codeword")
		}
	}
}

func TestPattersonRejectsTooManyErrors(t *testing.T) {
	params := mustParams(t, 8, 8)
	code, err := NewGoppaCode(params, deterministicRandom(24))
	if err != nil {
		t.Fatalf("NewGoppaCode: %v", err)
	}

	codeword := code.Matrix.MulVector(newVector(params.K))
	errorsVector, err := randomErrorVector(params.N, params.T+1, deterministicRandom(25))
	if err != nil {
		t.Fatalf("randomErrorVector: %v", err)
	}
	received := make([]uint64, len(codeword))
	for i := range codeword {
		received[i] = codeword[i] ^ errorsVector[i]
	}

	corrected, err := code.Decoder().Decode(received)
	if err == nil {
		same := true
		for i := range codeword {
			if corrected[i] != codeword[i] {
				same = false
				break
			}
		}
		if same {
			t.Fatal("decoder recovered the original codeword from t+1 errors")
		}
		return
	}
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("Decode error = %v, want ErrDecode", err)
	}
}

func TestDecoderRejectsShortWord(t *testing.T) {
	params := mustParams(t, 8, 8)
	code, err := NewGoppaCode(params, deterministicRandom(26))
	if err != nil {
		t.Fatalf("NewGoppaCode: %v", err)
	}
	if _, err := code.Decoder().Decode(newVector(params.N / 2)); !errors.Is(err, ErrDecode) {
		t.Fatalf("Decode error = %v, want ErrDecode", err)
	}
}
