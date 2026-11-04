package cipher

import (
	"bytes"
	"errors"
	"testing"
)

func TestElGamalKeySizesAndRoundTrip(t *testing.T) {
	elgamal := NewElGamal()

	tests := []struct {
		label      string
		option     KeyOption
		publicLen  int
		privateLen int
	}{
		{"256-bit curve", KeyOption{Label: "256-bit curve", Bits: 256}, 64, 32},
		{"512-bit curve", KeyOption{Label: "512-bit curve", Bits: 512}, 128, 64},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			keys, err := elgamal.GenerateKeys(tt.option)
			if err != nil {
				t.Fatalf("GenerateKeys: %v", err)
			}
			if len(keys.Public) != tt.publicLen || len(keys.Private) != tt.privateLen {
				t.Fatalf("ключи %d/%d байт, ожидалось %d/%d",
					len(keys.Public), len(keys.Private), tt.publicLen, tt.privateLen)
			}

			msg := []byte("adapter round trip")
			cipherText, err := elgamal.Encrypt(keys.Public, msg)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if len(cipherText) <= len(msg) {
				t.Fatalf("шифртекст %d байт не длиннее сообщения", len(cipherText))
			}

			plain, err := elgamal.Decrypt(keys.Private, cipherText)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if !bytes.Equal(plain, msg) {
				t.Fatalf("получено %q, ожидалось %q", plain, msg)
			}
		})
	}
}

func TestElGamalRejectsBadKeySizes(t *testing.T) {
	elgamal := NewElGamal()

	if _, err := elgamal.GenerateKeys(KeyOption{Bits: 128}); !errors.Is(err, ErrUnsupportedKeySize) {
		t.Fatalf("GenerateKeys(128): получено %v", err)
	}
	if _, err := elgamal.Encrypt(make([]byte, 32), []byte("x")); !errors.Is(err, ErrUnsupportedKeySize) {
		t.Fatalf("Encrypt с ключом 32 байта: получено %v", err)
	}
	if _, err := elgamal.Decrypt(make([]byte, 48), []byte("x")); !errors.Is(err, ErrUnsupportedKeySize) {
		t.Fatalf("Decrypt с ключом 48 байт: получено %v", err)
	}
}
