package gost3410

import (
	"bytes"
	"crypto/rand"
	"hash"
	"math/big"
	"testing"

	"go-cipher/internal/crypto/ec"
	gosthash "go-cipher/internal/crypto/hash"
)

// fixedHash отдаёт заранее заданный хеш-код. Он нужен для воспроизведения
// примера RFC 7091, где значение e приведено готовым, а сообщение и сама
// хеш-функция не указаны.
type fixedHash struct {
	digest []byte
}

func (h *fixedHash) Write(p []byte) (int, error) { return len(p), nil }
func (h *fixedHash) Sum(b []byte) []byte         { return append(b, h.digest...) }
func (h *fixedHash) Reset()                      {}
func (h *fixedHash) Size() int                   { return len(h.digest) }
func (h *fixedHash) BlockSize() int              { return 64 }

func hashFunc(digest []byte) func() hash.Hash {
	return func() hash.Hash { return &fixedHash{digest: digest} }
}

func hexInt(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 16)
	if !ok {
		t.Fatalf("некорректное hex-число %q", s)
	}
	return n
}

// rfc7091Curve — тестовая кривая из RFC 7091, разд. 7.1. Она не входит в
// стандартные наборы параметров и используется только для проверки
// арифметики и формул подписи.
func rfc7091Curve(t *testing.T) *ec.Curve {
	t.Helper()
	return &ec.Curve{
		Name: "RFC 7091 test curve",
		P:    hexInt(t, "8000000000000000000000000000000000000000000000000000000000000431"),
		A:    big.NewInt(7),
		B:    hexInt(t, "5FBFF498AA938CE739B8E022FBAFEF40563F6E6A3472FC2A514C0CE9DAE23B7E"),
		N:    hexInt(t, "8000000000000000000000000000000150FE8A1892976154C59CFC193ACCF5B3"),
		GX:   big.NewInt(2),
		GY:   hexInt(t, "8E2A8A0E65147D4BD6316030E16D19C85C97F0A9CA267122B96ABBCEA7E8FC8"),
	}
}

func TestRFC7091Example(t *testing.T) {
	c := rfc7091Curve(t)
	if err := c.Validate(); err != nil {
		t.Fatalf("параметры RFC 7091: %v", err)
	}

	d := hexInt(t, "7A929ADE789BB9BE10ED359DD39A72C11B60961F49397EEE1D19CE9891EC3B28")
	k := hexInt(t, "77105C9B20BCD3122823C8CF6FCC7B956DE33814E95B7FE64FED924594DCEAB3")
	e := hexInt(t, "2DFBC1B372D89A1188C09C52E0EEC61FCE52032AB1022E8E67ECE6672B043EE5")

	wantQ := ec.Point{
		X: hexInt(t, "7F2B49E270DB6D90D8595BEC458B50C58585BA1D4E9B788F6689DBD8E56FD80B"),
		Y: hexInt(t, "26F1B489D6701DD185C8413A977B3CBBAF64D1C593D26627DFFB101A87FF77DA"),
	}
	wantR := hexInt(t, "41AA28D2F1AB148280CD9ED56FEDA41974053554A42767B83AD043FD39DC0493")
	wantS := hexInt(t, "1456C64BA4642A1653C235A98A60249BCD6D3F746B631DF928014F6C5BF9C40")

	pub := c.ScalarMult(d, c.Base())
	if !pub.Equal(wantQ) {
		t.Fatalf("Q = d*G не совпал с примером:\nполучено %v\nожидалось %v", pub, wantQ)
	}

	// e в примере — уже готовое значение, поэтому хеш-функция подменяется
	// заглушкой, возвращающей его байтовую запись.
	eBytes := make([]byte, 64)
	e.FillBytes(eBytes)
	hashAlg := hashFunc(eBytes)

	nonce := make([]byte, 64)
	k.FillBytes(nonce)

	r, s, err := Sign(c, hashAlg, d, []byte("RFC 7091 test message"), bytes.NewReader(nonce))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if r.Cmp(wantR) != 0 {
		t.Fatalf("r = %X, ожидалось %X", r, wantR)
	}
	if s.Cmp(wantS) != 0 {
		t.Fatalf("s = %X, ожидалось %X", s, wantS)
	}

	ok, err := Verify(c, hashAlg, pub, []byte("RFC 7091 test message"), r, s)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("подпись из примера RFC 7091 не прошла проверку")
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		curve   *ec.Curve
		hashAlg func() hash.Hash
	}{
		{"256-битная кривая", ec.Params256(), gosthash.NewGOST3411_256},
		{"512-битная кривая", ec.Params512(), gosthash.NewGOST3411_512},
	}

	messages := [][]byte{
		[]byte(""),
		[]byte("MZI lab 6: цифровая подпись по ГОСТ Р 34.10-2012"),
		bytes.Repeat([]byte{0x00, 0xff, 0x55, 0xaa}, 128),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priv, pub, err := GenerateKey(tt.curve)
			if err != nil {
				t.Fatalf("GenerateKey: %v", err)
			}
			if priv.Sign() <= 0 || priv.Cmp(tt.curve.N) >= 0 {
				t.Fatalf("закрытый ключ вне диапазона: %X", priv)
			}
			if !tt.curve.IsOnCurve(pub) {
				t.Fatal("открытый ключ не лежит на кривой")
			}
			if !tt.curve.ScalarMult(tt.curve.N, pub).IsInfinity() {
				t.Fatal("порядок открытого ключа не равен q")
			}

			for _, msg := range messages {
				r, s, err := Sign(tt.curve, tt.hashAlg, priv, msg, rand.Reader)
				if err != nil {
					t.Fatalf("Sign: %v", err)
				}
				if r.Sign() <= 0 || r.Cmp(tt.curve.N) >= 0 || s.Sign() <= 0 || s.Cmp(tt.curve.N) >= 0 {
					t.Fatalf("r или s вне диапазона: r=%X s=%X", r, s)
				}

				ok, err := Verify(tt.curve, tt.hashAlg, pub, msg, r, s)
				if err != nil {
					t.Fatalf("Verify: %v", err)
				}
				if !ok {
					t.Fatal("корректная подпись отвергнута")
				}
			}
		})
	}
}

func TestTamperedInputsRejected(t *testing.T) {
	c := ec.Params256()

	tests := []struct {
		name    string
		message string
	}{
		{"изменено сообщение", "MZI lab 6: проверка целостности"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priv, pub, err := GenerateKey(c)
			if err != nil {
				t.Fatalf("GenerateKey: %v", err)
			}

			signed := []byte("MZI lab 6: исходное сообщение")
			r, s, err := Sign(c, gosthash.NewGOST3411_256, priv, signed, rand.Reader)
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}

			ok, err := Verify(c, gosthash.NewGOST3411_256, pub, []byte(tt.message), r, s)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok {
				t.Fatal("подпись чужого сообщения признана верной")
			}

			// Повреждённое s.
			badS := new(big.Int).Add(s, big.NewInt(1))
			badS.Mod(badS, c.N)
			ok, err = Verify(c, gosthash.NewGOST3411_256, pub, signed, r, badS)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok {
				t.Fatal("повреждённая подпись признана верной")
			}

			// Чужой открытый ключ.
			_, otherPub, err := GenerateKey(c)
			if err != nil {
				t.Fatalf("GenerateKey: %v", err)
			}
			ok, err = Verify(c, gosthash.NewGOST3411_256, otherPub, signed, r, s)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok {
				t.Fatal("подпись проверена чужим ключом")
			}
		})
	}
}

func TestOutOfRangeSignatureRejected(t *testing.T) {
	c := ec.Params256()
	priv, pub, err := GenerateKey(c)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	msg := []byte("граничные значения r и s")
	r, s, err := Sign(c, gosthash.NewGOST3411_256, priv, msg, rand.Reader)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	values := []*big.Int{
		big.NewInt(0),
		big.NewInt(-1),
		new(big.Int).Set(c.N),
		new(big.Int).Add(c.N, big.NewInt(1)),
	}

	for _, v := range values {
		for _, pair := range [][2]*big.Int{{v, s}, {r, v}} {
			ok, err := Verify(c, gosthash.NewGOST3411_256, pub, msg, pair[0], pair[1])
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok {
				t.Fatalf("подпись с r=%X, s=%X признана верной", pair[0], pair[1])
			}
		}
	}
}

func TestSignRejectsBadPrivateKey(t *testing.T) {
	c := ec.Params256()
	msg := []byte("x")

	for _, d := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1), new(big.Int).Set(c.N)} {
		if _, _, err := Sign(c, gosthash.NewGOST3411_256, d, msg, rand.Reader); err == nil {
			t.Fatalf("закрытый ключ %v принят", d)
		}
	}
}

func TestVerifyRejectsInvalidPublicKey(t *testing.T) {
	c := ec.Params256()
	msg := []byte("x")
	r, s := big.NewInt(1), big.NewInt(1)

	tests := []struct {
		name string
		pub  ec.Point
	}{
		{"бесконечно удалённая точка", ec.Point{}},
		{"точка не на кривой", ec.Point{X: big.NewInt(1), Y: big.NewInt(1)}},
	}

	for _, tt := range tests {
		if _, err := Verify(c, gosthash.NewGOST3411_256, tt.pub, msg, r, s); err == nil {
			t.Fatalf("%s: ожидалась ошибка", tt.name)
		}
	}
}

func TestDeterministicNonce(t *testing.T) {
	c := ec.Params256()
	priv, _, err := GenerateKey(c)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	nonce := make([]byte, 32)
	big.NewInt(0x1234567).FillBytes(nonce)
	msg := []byte("одно и то же k даёт одну и ту же подпись")

	r1, s1, err := Sign(c, gosthash.NewGOST3411_256, priv, msg, bytes.NewReader(nonce))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	r2, s2, err := Sign(c, gosthash.NewGOST3411_256, priv, msg, bytes.NewReader(nonce))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if r1.Cmp(r2) != 0 || s1.Cmp(s2) != 0 {
		t.Fatal("при одинаковом k получены разные подписи")
	}
}
