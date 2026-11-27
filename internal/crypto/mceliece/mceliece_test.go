package mceliece

import (
	"bytes"
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	binaryMessage := make([]byte, 1024)
	reader := deterministicRandom(31)
	for i := range binaryMessage {
		if _, err := reader.Read(binaryMessage[i : i+1]); err != nil {
			t.Fatalf("filling message: %v", err)
		}
	}

	messages := []struct {
		name string
		data []byte
	}{
		{"пустое сообщение", nil},
		{"один байт", []byte{0x42}},
		{"нулевые байты", []byte{0, 0, 0, 0}},
		{"несколько блоков", bytes.Repeat([]byte("McEliece "), 100)},
		{"двоичные данные", binaryMessage},
	}

	for _, preset := range [][2]int{{8, 8}, {10, 38}} {
		params := mustParams(t, preset[0], preset[1])
		public, private, err := GenerateKeyPair(params, deterministicRandom(32))
		if err != nil {
			t.Fatalf("params %v: GenerateKeyPair: %v", preset, err)
		}

		blockBytes := params.N / 8
		for _, message := range messages {
			cipher, err := public.EncryptWithRandom(message.data, deterministicRandom(33))
			if err != nil {
				t.Fatalf("params %v, %s: Encrypt: %v", preset, message.name, err)
			}
			totalBits := framingBits + 8*len(message.data)
			blocks := (totalBits + params.K - 1) / params.K
			if len(cipher) != blocks*blockBytes {
				t.Fatalf("params %v, %s: ciphertext %d B, want %d B",
					preset, message.name, len(cipher), blocks*blockBytes)
			}

			plain, err := private.Decrypt(cipher)
			if err != nil {
				t.Fatalf("params %v, %s: Decrypt: %v", preset, message.name, err)
			}
			if !bytes.Equal(plain, message.data) {
				t.Fatalf("params %v, %s: plaintext differs (%d B, want %d B)",
					preset, message.name, len(plain), len(message.data))
			}
		}
	}
}

func TestEncryptionIsRandomized(t *testing.T) {
	params := mustParams(t, 8, 8)
	public, _, err := GenerateKeyPair(params, deterministicRandom(34))
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	message := []byte("one block")
	first, err := public.EncryptWithRandom(message, deterministicRandom(35))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	second, err := public.EncryptWithRandom(message, deterministicRandom(36))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two encryptions of the same message are identical")
	}
}

func TestDecryptRejectsBadInput(t *testing.T) {
	params := mustParams(t, 8, 8)
	public, private, err := GenerateKeyPair(params, deterministicRandom(37))
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	otherPublic, otherPrivate, err := GenerateKeyPair(params, deterministicRandom(38))
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	_ = otherPublic

	cipher, err := public.EncryptWithRandom([]byte("секрет"), deterministicRandom(39))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := private.Decrypt(cipher[:len(cipher)-1]); !errors.Is(err, ErrBadCipher) {
		t.Fatalf("truncated ciphertext: error = %v, want ErrBadCipher", err)
	}
	if _, err := private.Decrypt(nil); !errors.Is(err, ErrBadCipher) {
		t.Fatalf("empty ciphertext: error = %v, want ErrBadCipher", err)
	}
	if _, err := private.Decrypt(append(append([]byte(nil), cipher...), make([]byte, params.N/8)...)); !errors.Is(err, ErrBadCipher) {
		t.Fatalf("extra block: error = %v, want ErrBadCipher", err)
	}
	if _, err := otherPrivate.Decrypt(cipher); err == nil {
		t.Fatal("decryption with a foreign private key succeeded")
	}
}

func TestEncryptRejectsCorruptedPublicKey(t *testing.T) {
	params := mustParams(t, 8, 8)
	public, _, err := GenerateKeyPair(params, deterministicRandom(40))
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	data := public.MarshalBinary()

	corrupted := append([]byte(nil), data...)
	corrupted[0] ^= 0xFF
	if _, err := ParsePublicKey(corrupted); !errors.Is(err, ErrBadPublicKey) {
		t.Fatalf("bad magic: error = %v, want ErrBadPublicKey", err)
	}
	if _, err := ParsePublicKey(data[:len(data)-1]); !errors.Is(err, ErrBadPublicKey) {
		t.Fatalf("truncated key: error = %v, want ErrBadPublicKey", err)
	}
}

func TestKeySerializationRoundTrip(t *testing.T) {
	params := mustParams(t, 8, 8)
	public, private, err := GenerateKeyPair(params, deterministicRandom(41))
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	publicBytes := public.MarshalBinary()
	parsedPublic, err := ParsePublicKey(publicBytes)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}
	if !parsedPublic.Matrix.Equal(public.Matrix) || parsedPublic.Params != public.Params {
		t.Fatal("public key changed after round trip")
	}
	if !bytes.Equal(parsedPublic.MarshalBinary(), publicBytes) {
		t.Fatal("public key is not byte-exact after round trip")
	}

	privateBytes := private.MarshalBinary()
	parsedPrivate, err := ParsePrivateKey(privateBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKey: %v", err)
	}
	if !parsedPrivate.SInverse.Equal(private.SInverse) ||
		!samePoly(parsedPrivate.G, private.G) ||
		len(parsedPrivate.Support) != len(private.Support) {
		t.Fatal("private key changed after round trip")
	}
	if !bytes.Equal(parsedPrivate.MarshalBinary(), privateBytes) {
		t.Fatal("private key is not byte-exact after round trip")
	}
}

func TestParsePrivateKeyRejectsCorruptedSupport(t *testing.T) {
	params := mustParams(t, 8, 8)
	_, private, err := GenerateKeyPair(params, deterministicRandom(42))
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	data := private.MarshalBinary()

	supportOffset := privateHeaderSize + 2*len(private.G)
	corrupted := append([]byte(nil), data...)
	copy(corrupted[supportOffset:], corrupted[supportOffset+2:supportOffset+4])
	if _, err := ParsePrivateKey(corrupted); !errors.Is(err, ErrBadPrivateKey) {
		t.Fatalf("duplicate support element: error = %v, want ErrBadPrivateKey", err)
	}
	if _, err := ParsePrivateKey(data[:len(data)-3]); !errors.Is(err, ErrBadPrivateKey) {
		t.Fatalf("truncated key: error = %v, want ErrBadPrivateKey", err)
	}
}

func samePoly(a, b Poly) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
