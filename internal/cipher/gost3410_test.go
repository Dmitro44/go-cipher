package cipher

import (
	"bytes"
	"testing"
)

func TestGost3410RoundTrip(t *testing.T) {
	scheme := NewGost3410()
	options := scheme.KeyOptions()

	for _, option := range options {
		t.Run(option.Label, func(t *testing.T) {
			keys, err := scheme.GenerateKeys(option)
			if err != nil {
				t.Fatalf("GenerateKeys: %v", err)
			}
			if len(keys.Public) != 2*len(keys.Private) {
				t.Fatalf("public key %d bytes, private %d", len(keys.Public), len(keys.Private))
			}

			msg := []byte("MZI lab 6: сообщение для подписи")
			sig, err := scheme.Sign(keys.Private, msg)
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if len(sig) != len(keys.Public) {
				t.Fatalf("signature %d bytes, expected %d", len(sig), len(keys.Public))
			}

			ok, err := scheme.Verify(keys.Public, msg, sig)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if !ok {
				t.Fatal("корректная подпись отвергнута")
			}

			tampered := bytes.Clone(msg)
			tampered[0] ^= 1
			ok, err = scheme.Verify(keys.Public, tampered, sig)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok {
				t.Fatal("подпись изменённого сообщения признана верной")
			}
		})
	}
}

func TestGost3410RejectsWrongSizes(t *testing.T) {
	scheme := NewGost3410()

	if _, err := scheme.GenerateKeys(KeyOption{Bits: 1024}); err == nil {
		t.Fatal("неподдерживаемый размер ключа принят")
	}
	if _, err := scheme.Sign([]byte{1, 2, 3}, []byte("x")); err == nil {
		t.Fatal("закрытый ключ неправильной длины принят")
	}
	if _, err := scheme.Verify([]byte{1, 2, 3}, []byte("x"), nil); err == nil {
		t.Fatal("открытый ключ неправильной длины принят")
	}

	keys, err := scheme.GenerateKeys(KeyOption{Bits: 256})
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}
	if _, err := scheme.Verify(keys.Public, []byte("x"), make([]byte, 10)); err == nil {
		t.Fatal("подпись неправильной длины принята")
	}
}
