package hash

// Эталонные примеры ГОСТ Р 34.11-2012 из RFC 6986, п. 10.

import (
	"bytes"
	"encoding/hex"
	"hash"
	"strings"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatalf("некорректная hex-строка: %v", err)
	}
	return b
}

func mustState(t *testing.T, s string) [64]byte {
	t.Helper()
	raw := mustHex(t, s)
	if len(raw) != 64 {
		t.Fatalf("вектор длиной %d байт вместо 64", len(raw))
	}
	return stateFromBigEndian(raw)
}

func stateHex(a [64]byte) string {
	raw := make([]byte, 64)
	for i := range raw {
		raw[i] = a[63-i]
	}
	return hex.EncodeToString(raw)
}

// repeatHex повторяет 16-ричный байт нужное число раз.
func repeatHex(s string, n int) string {
	return strings.Repeat(s, n)
}

// rfcMessage возвращает байты сообщения из векторной записи RFC 6986:
// в стандарте вектор записан старшим байтом вперёд, поэтому байты сообщения
// идут в обратном порядке.
func rfcMessage(t *testing.T, vectorHex string) []byte {
	t.Helper()
	raw := mustHex(t, vectorHex)
	message := make([]byte, len(raw))
	for i := range raw {
		message[i] = raw[len(raw)-1-i]
	}
	return message
}

// M1 и M2 из RFC 6986, п. 10: векторы сообщений заданы старшим байтом вперёд.
const (
	rfcM1 = "32313039383736353433323130393837" +
		"36353433323130393837363534333231" +
		"30393837363534333231303938373635" +
		"343332313039383736353433323130"

	rfcM2 = "fbe2e5f0eee3c820fbeafaebef20fffb" +
		"f0e1e0f0f520e0ed20e8ece0ebe5f0f2" +
		"f120fff0eeec20f120faf2fee5e2202c" +
		"e8f6f3ede220e8e6eee1e8f0f2d1202c" +
		"e8f0f2e5e220e5d1"
)

// Пример 1, п. 10.1.1: промежуточные значения для h = N = 0 и M1.
func TestGOST3411Example1Intermediates(t *testing.T) {
	zero := [64]byte{}
	m := mustState(t, "01"+rfcM1)

	s := S(zero)
	if got, want := stateHex(s), repeatHex("fc", 64); got != want {
		t.Fatalf("S(0):\n получено %s\n ожидалось %s", got, want)
	}

	if got, want := stateHex(P(s)), repeatHex("fc", 64); got != want {
		t.Fatalf("P(S(0)):\n получено %s\n ожидалось %s", got, want)
	}

	k := lps(zero)
	wantK := repeatHex("b383fc2eced4a574", 8)
	if got := stateHex(k); got != wantK {
		t.Fatalf("K = LPS(0):\n получено %s\n ожидалось %s", got, wantK)
	}

	x1 := X(k, m)
	if got, want := stateHex(x1), "b2b1cd1ef7ec924286b7cf1cffe49c4c84b5c91afde694448abbcb18fbe0964682b3c516f9e2904080b1cd1ef7ec924286b7cf1cffe49c4c84b5c91afde69444"; got != want {
		t.Fatalf("X[K[1]](m):\n получено %s\n ожидалось %s", got, want)
	}

	sx := S(x1)
	if got, want := stateHex(sx), "4645d95fc0beec2c432f8914b62d4efd3e5e37f14b097aead67de417c220b0482492ac996667e0ebdf45d95fc0beec2c432f8914b62d4efd3e5e37f14b097aea"; got != want {
		t.Fatalf("SX[K[1]](m):\n получено %s\n ожидалось %s", got, want)
	}

	psx := P(sx)
	if got, want := stateHex(psx), "46433ed624df433e452f5e7d92452f5ed98937e4acd989375f14f117995f14f1c0b64bc266c0b64bbe2d092067be2d09ec4e7ab0e0ec4e7a2cfdea48eb2cfdea"; got != want {
		t.Fatalf("PSX[K[1]](m):\n получено %s\n ожидалось %s", got, want)
	}

	lpsx := lps(x1)
	if got, want := stateHex(lpsx), "e60059d4d8e0758024c73f6f3183653f56579189602ae4c21e7953ebc0e212a0ce78a8df475c2fd4fc43fc4b71c01e35be465fb20dad2cf690cdf65028121bb9"; got != want {
		t.Fatalf("LPSX[K[1]](m):\n получено %s\n ожидалось %s", got, want)
	}

	kc := X(k, iterationConstants[0])
	if got, want := stateHex(kc), "028ba7f4d01e7f9d5848d3af0eb1d96b9ce98a6de0917562c2cd44a3bb516188f8ff1cbf5cb3cc7511c1d6266ab47661b6f5881802a0e8576e0399773c72e073"; got != want {
		t.Fatalf("K[1] xor C[1]:\n получено %s\n ожидалось %s", got, want)
	}

	skc := S(kc)
	if got, want := stateHex(skc), "ddf644e6e15f5733bff249410445536f4e9bd69e200f3596b3d9ea737d70a1d7d1b6143b9c9288357758f8ef78278aa155f4d717dda7cb12b211e87e7f19203d"; got != want {
		t.Fatalf("S(K[1] xor C[1]):\n получено %s\n ожидалось %s", got, want)
	}

	pskc := P(skc)
	if got, want := stateHex(pskc), "ddbf4eb3d17755b2f6f29bd9b658f4114449d6ea14f8d7e8e6419e733bef177ee104207d9c78dd7f5f450f709227a719575335a1888acb20336f96d735a1123d"; got != want {
		t.Fatalf("PS(K[1] xor C[1]):\n получено %s\n ожидалось %s", got, want)
	}

	k2 := keySchedule(k, 1)
	if got, want := stateHex(k2), "d0b00807642fd78f13f2c3ebc774e80de0e902d23aef2ee9a73d010807dae9c188be14f0b2da27973569cd2ba051301036f728bd1d7eec33f4d18af70c46cf1e"; got != want {
		t.Fatalf("K[2] = LPS(K[1] xor C[1]):\n получено %s\n ожидалось %s", got, want)
	}

	h1 := gN(zero, m, zero)
	if got, want := stateHex(h1), "fd102cf8812ccb1191ea34af21394f3817a86641445aa9a626488adb33738ebd2754f6908cbbbac5d3ed0f522c50815c954135793fb1f5d905fee4736b3bdae2"; got != want {
		t.Fatalf("g_N(0, m):\n получено %s\n ожидалось %s", got, want)
	}

	n := addBits(zero, 504)
	if got, want := stateHex(n), "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001f8"; got != want {
		t.Fatalf("N после дополнения:\n получено %s\n ожидалось %s", got, want)
	}

	h2 := gN(h1, n, zero)
	if got, want := stateHex(h2), "5c881fd924695cf196c2e4fec20d14b642026f2a0b1716ebaabb7067d4d597523d2db69d6d3794622147a14f19a66e7f9037e1d662d34501a8901a5de7771d7c"; got != want {
		t.Fatalf("g_0(h, N):\n получено %s\n ожидалось %s", got, want)
	}

	h3 := gN(h2, m, zero)
	if got, want := stateHex(h3), "486f64c1917879417fef082b3381a4e211c324f074654c38823a7b76f830ad00fa1fbae42b1285c0352f227524bc9ab16254288dd6863dccd5b9f54a1ad0541b"; got != want {
		t.Fatalf("H(M1):\n получено %s\n ожидалось %s", got, want)
	}
}

// Пример 1, п. 10.1.2: первый ключ для 256-битного варианта.
func TestGOST3411Example1InitialKey256(t *testing.T) {
	var iv [64]byte
	for i := range iv {
		iv[i] = 0x01
	}
	if got, want := stateHex(lps(iv)), repeatHex("23c5ee40b07b5f15", 8); got != want {
		t.Fatalf("K = LPS((00000001)^64):\n получено %s\n ожидалось %s", got, want)
	}
}

// Эталонные дайджесты из RFC 6986, п. 10.1.1, 10.1.2, 10.2.1, 10.2.2.
func TestGOST3411RFCExamples(t *testing.T) {
	cases := []struct {
		name   string
		new    func() hash.Hash
		vector string
		want   string
	}{
		{"M1 512", NewGOST3411_512, rfcM1, "486f64c1917879417fef082b3381a4e211c324f074654c38823a7b76f830ad00fa1fbae42b1285c0352f227524bc9ab16254288dd6863dccd5b9f54a1ad0541b"},
		{"M1 256", NewGOST3411_256, rfcM1, "00557be5e584fd52a449b16b0251d05d27f94ab76cbaa6da890b59d8ef1e159d"},
		{"M2 512", NewGOST3411_512, rfcM2, "28fbc9bada033b1460642bdcddb90c3fb3e56c497ccd0f62b8a2ad4935e85f037613966de4ee00531ae60f3b5a47f8dae06915d5f2f194996fcabf2622e6881e"},
		{"M2 256", NewGOST3411_256, rfcM2, "508f7e553c06501d749a66fc28c6cac0b005746d97537fa85d9e40904efed29d"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.new()
			if _, err := h.Write(rfcMessage(t, tc.vector)); err != nil {
				t.Fatal(err)
			}
			if got := hex.EncodeToString(h.Sum(nil)); got != tc.want {
				t.Fatalf("получено %s\nожидалось  %s", got, tc.want)
			}
		})
	}
}

func TestGOST3411Incremental(t *testing.T) {
	message := make([]byte, 300)
	for i := range message {
		message[i] = byte(i*7 + 3)
	}

	for _, alg := range Algorithms[:2] {
		t.Run(alg.Name, func(t *testing.T) {
			oneShot := alg.New()
			oneShot.Write(message)
			want := hex.EncodeToString(oneShot.Sum(nil))

			for _, chunk := range []int{1, 3, 7, 63, 64, 65, 128} {
				h := alg.New()
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
		})
	}
}

func TestGOST3411Reset(t *testing.T) {
	h := NewGOST3411_512()
	h.Write([]byte("first"))
	first := h.Sum(nil)

	h.Reset()
	h.Write([]byte("second"))
	second := h.Sum(nil)

	again := NewGOST3411_512()
	again.Write([]byte("second"))
	if !bytes.Equal(second, again.Sum(nil)) {
		t.Fatal("после Reset хеш-функция не вернулась в начальное состояние")
	}
	if bytes.Equal(first, second) {
		t.Fatal("разные сообщения дали одинаковый дайджест")
	}
}
