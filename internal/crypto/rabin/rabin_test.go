package rabin

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"strings"
	"testing"
)

func TestLabExampleRoots(t *testing.T) {
	p, q := big.NewInt(7), big.NewInt(11)
	n := big.NewInt(77)
	m := big.NewInt(20)

	c := new(big.Int).Exp(m, two, n)
	if c.Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("20^2 mod 77 = %s, want 15", c)
	}

	want := map[int64]bool{64: true, 20: true, 13: true, 57: true}
	roots := Roots(c, p, q)
	if len(roots) != 4 {
		t.Fatalf("got %d roots, want 4", len(roots))
	}
	for _, r := range roots {
		if !want[r.Int64()] {
			t.Errorf("root %s not in lab set {64, 20, 13, 57}", r)
		}
		if new(big.Int).Exp(r, two, n).Cmp(c) != 0 {
			t.Errorf("root %s does not square to %s mod 77", r, c)
		}
	}
}

func TestOnlyTrueRootCarriesRedundancy(t *testing.T) {
	p, q, err := GenerateKeys(512)
	if err != nil {
		t.Fatal(err)
	}
	n := new(big.Int).Mul(p, q)
	payload := []byte("block payload")
	m := new(big.Int).SetBytes(pad(payload, blockLen(n)))
	c := new(big.Int).Exp(m, two, n)

	var valid []*big.Int
	for _, r := range Roots(c, p, q) {
		if _, ok := blockPayload(r, blockLen(n)); ok {
			valid = append(valid, r)
		}
	}
	if len(valid) != 1 || valid[0].Cmp(m) != 0 {
		t.Fatalf("valid roots = %v, want exactly the original block %s", valid, m)
	}
}

func TestRoundTrip(t *testing.T) {
	p, q, err := GenerateKeys(512)
	if err != nil {
		t.Fatal(err)
	}
	n := new(big.Int).Mul(p, q)
	if n.BitLen() != 512 {
		t.Fatalf("modulus has %d bits, want 512", n.BitLen())
	}

	random := make([]byte, 300)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}

	inputs := map[string][]byte{
		"empty":      {},
		"short":      []byte("hi"),
		"multiBlock": []byte(strings.Repeat("Криптосистема Рабина. ", 40)),
		"zeroTail":   append([]byte("data"), 0, 0, 0),
		"binary":     random,
	}
	for name, msg := range inputs {
		ct, err := Encrypt(n, msg)
		if err != nil {
			t.Fatalf("%s: encrypt: %v", name, err)
		}
		got, err := Decrypt(p, q, ct)
		if err != nil {
			t.Fatalf("%s: decrypt: %v", name, err)
		}
		if !bytes.Equal(got, msg) {
			t.Errorf("%s: round trip mismatch: got %d bytes, want %d", name, len(got), len(msg))
		}
	}
}

func TestDecryptRejectsBadInput(t *testing.T) {
	p, q, err := GenerateKeys(512)
	if err != nil {
		t.Fatal(err)
	}
	n := new(big.Int).Mul(p, q)
	ct, err := Encrypt(n, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Decrypt(p, q, ct[:len(ct)-1]); err == nil {
		t.Error("truncated ciphertext accepted")
	}
	if _, err := Decrypt(p, p, ct); err != ErrInvalidKey {
		t.Errorf("equal primes: err = %v, want ErrInvalidKey", err)
	}
}
