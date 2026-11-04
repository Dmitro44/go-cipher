package ecelgamal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"math/rand"
	"testing"

	"go-cipher/internal/crypto/ec"
)

// params — оба набора параметров ГОСТ Р 34.10-2012, на которых работает
// схема.
func params() []*ec.Curve {
	return []*ec.Curve{ec.Params256(), ec.Params512()}
}

// testKey строит ключевую пару с заданным малым закрытым ключом, чтобы
// тесты были детерминированными.
func testKey(t *testing.T, c *ec.Curve, d int64) (*big.Int, ec.Point) {
	t.Helper()
	priv := big.NewInt(d)
	return priv, c.ScalarMult(priv, c.Base())
}

// fixedReader возвращает одинаковые байты при каждом обращении: множитель
// k получается одним и тем же, поэтому шифртекст детерминирован.
func fixedReader() io.Reader {
	return bytes.NewReader(bytes.Repeat([]byte{0x01}, 1<<16))
}

func TestParamSetsSupportShortSquareRoot(t *testing.T) {
	wantChunk := map[int]int{256: 31, 512: 63}
	wantWidth := map[int]int{256: 32, 512: 64}

	for _, c := range params() {
		t.Run(c.Name, func(t *testing.T) {
			if !primeIs3Mod4(c.P) {
				t.Fatalf("p mod 4 = %s, нужен 3", new(big.Int).Mod(c.P, big.NewInt(4)))
			}
			if got := ChunkBytes(c); got != wantChunk[c.P.BitLen()] {
				t.Fatalf("ChunkBytes = %d, ожидалось %d", got, wantChunk[c.P.BitLen()])
			}
			if got := coordinateWidth(c); got != wantWidth[c.P.BitLen()] {
				t.Fatalf("coordinateWidth = %d, ожидалось %d", got, wantWidth[c.P.BitLen()])
			}
		})
	}

	if primeIs3Mod4(big.NewInt(5)) {
		t.Fatal("5 mod 4 = 1, функция должна вернуть false")
	}
}

func TestChunkBoundKeepsMappingInField(t *testing.T) {
	for _, c := range params() {
		t.Run(c.Name, func(t *testing.T) {
			chunkBytes := ChunkBytes(c)
			// Наибольший возможный фрагмент — все байты равны 0xFF.
			m := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(8*chunkBytes)), big.NewInt(1))
			maxX := new(big.Int).Mul(m, big.NewInt(K))
			maxX.Add(maxX, big.NewInt(K-1))
			if maxX.Cmp(c.P) >= 0 {
				t.Fatalf("m*K + j = %s не меньше p", maxX)
			}
		})
	}
}

func TestMessageMappingCoversEveryByte(t *testing.T) {
	for _, c := range params() {
		t.Run(c.Name, func(t *testing.T) {
			chunkBytes := ChunkBytes(c)
			for b := 0; b < 256; b++ {
				chunk := make([]byte, chunkBytes)
				chunk[chunkBytes-1] = byte(b)

				point, err := MessageToPoint(c, chunk)
				if err != nil {
					t.Fatalf("байт %#x: MessageToPoint: %v", b, err)
				}
				if !c.IsOnCurve(point) {
					t.Fatalf("байт %#x: точка не лежит на кривой", b)
				}

				// Абсцисса должна иметь вид m*K + j, j = 0..K-1.
				q, r := new(big.Int).QuoRem(point.X, big.NewInt(K), new(big.Int))
				if q.Cmp(big.NewInt(int64(b))) != 0 {
					t.Fatalf("байт %#x: m = %s", b, q)
				}
				if r.Cmp(big.NewInt(K)) >= 0 {
					t.Fatalf("байт %#x: j = %s", b, r)
				}

				back, err := PointToMessage(c, point, chunkBytes)
				if err != nil {
					t.Fatalf("байт %#x: PointToMessage: %v", b, err)
				}
				if !bytes.Equal(back, chunk) {
					t.Fatalf("байт %#x: восстановлено %x", b, back)
				}
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	binaryData := make([]byte, 200)
	rand.New(rand.NewSource(42)).Read(binaryData)

	cases := []struct {
		name string
		msg  []byte
	}{
		{"пустое сообщение", nil},
		{"один байт", []byte{0x42}},
		{"короткий текст", []byte("Эль-Гамаль на кривой")},
		{"нулевые байты", make([]byte, 10)},
		{"случайные двоичные данные", binaryData},
	}

	for _, c := range params() {
		priv, pub := testKey(t, c, 7)
		chunkBytes := ChunkBytes(c)

		// Границы фрагментации: полезная нагрузка включает 4 байта
		// заголовка.
		for _, n := range []int{chunkBytes - headerBytes, chunkBytes - headerBytes + 1, 3*chunkBytes - headerBytes + 5} {
			cases = append(cases, struct {
				name string
				msg  []byte
			}{name: "длина " + big.NewInt(int64(n)).String(), msg: bytes.Repeat([]byte{0xA5}, n)})
		}

		t.Run(c.Name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					ct, err := Encrypt(c, pub, tc.msg, fixedReader())
					if err != nil {
						t.Fatalf("Encrypt: %v", err)
					}
					got, err := Decrypt(c, priv, ct)
					if err != nil {
						t.Fatalf("Decrypt: %v", err)
					}
					if !bytes.Equal(got, tc.msg) {
						t.Fatalf("получено %x, ожидалось %x", got, tc.msg)
					}
				})
			}
		})

		cases = cases[:5]
	}
}

func TestCiphertextExpansion(t *testing.T) {
	for _, c := range params() {
		t.Run(c.Name, func(t *testing.T) {
			_, pub := testKey(t, c, 7)
			blockBytes := 4 * coordinateWidth(c)
			chunkBytes := ChunkBytes(c)

			for _, size := range []int{0, 1, chunkBytes - headerBytes, chunkBytes - headerBytes + 1, 1000} {
				msg := make([]byte, size)
				ct, err := Encrypt(c, pub, msg, fixedReader())
				if err != nil {
					t.Fatalf("Encrypt(%d): %v", size, err)
				}

				blocks := (headerBytes + size + chunkBytes - 1) / chunkBytes
				want := blocks * blockBytes
				if len(ct) != want {
					t.Fatalf("длина сообщения %d: шифртекст %d байт, ожидалось %d", size, len(ct), want)
				}
			}
		})
	}
}

func TestDeterministicNonce(t *testing.T) {
	c := ec.Params256()
	priv, pub := testKey(t, c, 11)
	msg := []byte("one nonce, one ciphertext")

	first, err := Encrypt(c, pub, msg, fixedReader())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	second, err := Encrypt(c, pub, msg, fixedReader())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("при одинаковом k шифртексты должны совпадать")
	}

	// Другой k даёт другой шифртекст: схема рандомизирована.
	other, err := Encrypt(c, pub, msg, bytes.NewReader(bytes.Repeat([]byte{0x02}, 1<<16)))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(first, other) {
		t.Fatal("при разных k шифртексты должны различаться")
	}

	got, err := Decrypt(c, priv, first)
	if err != nil || !bytes.Equal(got, msg) {
		t.Fatalf("Decrypt: %x, %v", got, err)
	}
}

func TestWrongKeyDoesNotRecoverMessage(t *testing.T) {
	for _, c := range params() {
		t.Run(c.Name, func(t *testing.T) {
			_, pub := testKey(t, c, 5)
			wrongPriv, _ := testKey(t, c, 6)
			msg := []byte("secret message for the right key only")

			ct, err := Encrypt(c, pub, msg, fixedReader())
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			got, err := Decrypt(c, wrongPriv, ct)
			if err == nil && bytes.Equal(got, msg) {
				t.Fatal("чужой закрытый ключ восстановил исходное сообщение")
			}
		})
	}
}

func TestDecryptRejectsMalformed(t *testing.T) {
	c := ec.Params256()
	priv, pub := testKey(t, c, 3)
	width := coordinateWidth(c)
	blockBytes := 4 * width

	valid, err := Encrypt(c, pub, []byte("malformed ciphertext"), fixedReader())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Портим ординату C1: точка перестаёт лежать на кривой.
	offCurve := append([]byte(nil), valid...)
	c1, _ := parseBlock(offCurve[:blockBytes], width)
	badY := new(big.Int).Add(c1.Y, big.NewInt(1))
	badY.Mod(badY, c.P)
	badY.FillBytes(offCurve[width : 2*width])

	// Нулевые координаты изображают бесконечно удалённую точку.
	infinity := append([]byte(nil), valid...)
	clear(infinity[:2*width])

	// Точка G вместо отображения фрагмента: абсцисса не имеет вид m*K + j.
	notMapped := appendPoint(nil, c.ScalarMult(big.NewInt(9), c.Base()), width)
	notMapped = appendPoint(notMapped, c.Add(c.Base(), c.ScalarMult(big.NewInt(9), pub)), width)

	// Заголовок объявляет длину больше, чем вмещает полезная нагрузка.
	chunk := make([]byte, ChunkBytes(c))
	binary.BigEndian.PutUint32(chunk[:headerBytes], 0xFFFFFFFF)
	huge, err := MessageToPoint(c, chunk)
	if err != nil {
		t.Fatalf("MessageToPoint: %v", err)
	}
	tooLong := appendPoint(nil, c.ScalarMult(big.NewInt(9), c.Base()), width)
	tooLong = appendPoint(tooLong, c.Add(huge, c.ScalarMult(big.NewInt(9), pub)), width)

	tests := []struct {
		name string
		ct   []byte
	}{
		{"пустой шифртекст", nil},
		{"обрезанный шифртекст", valid[:len(valid)-1]},
		{"длина не кратна блоку", append(append([]byte(nil), valid...), 0)},
		{"точка не на кривой", offCurve},
		{"бесконечно удалённая точка", infinity},
		{"точка не отображение фрагмента", notMapped},
		{"длина сообщения больше нагрузки", tooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decrypt(c, priv, tt.ct); err == nil {
				t.Fatal("повреждённый шифртекст принят")
			}
		})
	}
}

func TestInvalidKeysRejected(t *testing.T) {
	c := ec.Params256()
	priv, pub := testKey(t, c, 3)

	if _, err := Decrypt(c, big.NewInt(0), []byte{1}); !errors.Is(err, ErrPrivateKey) {
		t.Fatalf("нулевой ключ: получено %v", err)
	}
	if _, err := Decrypt(c, c.N, []byte{1}); !errors.Is(err, ErrPrivateKey) {
		t.Fatalf("ключ N: получено %v", err)
	}
	if _, err := Encrypt(c, ec.Point{}, []byte("x"), fixedReader()); !errors.Is(err, ErrPublicKey) {
		t.Fatalf("бесконечно удалённый ключ: получено %v", err)
	}
	if _, err := Encrypt(c, ec.Point{X: big.NewInt(1), Y: big.NewInt(1)}, []byte("x"), fixedReader()); !errors.Is(err, ErrPublicKey) {
		t.Fatalf("точка не на кривой: получено %v", err)
	}

	msg := []byte("key validation")
	ct, err := Encrypt(c, pub, msg, fixedReader())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt(c, priv, ct); err != nil {
		t.Fatalf("корректный ключ отвергнут: %v", err)
	}
}

func TestGenerateKey(t *testing.T) {
	for _, c := range params() {
		t.Run(c.Name, func(t *testing.T) {
			priv, pub, err := GenerateKey(c, fixedReader())
			if err != nil {
				t.Fatalf("GenerateKey: %v", err)
			}
			if priv.Sign() <= 0 || priv.Cmp(c.N) >= 0 {
				t.Fatalf("d = %s вне диапазона", priv)
			}
			if !pub.Equal(c.ScalarMult(priv, c.Base())) {
				t.Fatal("Q != d*G")
			}
			if !c.IsOnCurve(pub) {
				t.Fatal("открытый ключ не лежит на кривой")
			}
		})
	}
}

func TestMessageChunkTooLongRejected(t *testing.T) {
	c := ec.Params256()
	chunk := make([]byte, ChunkBytes(c)+1)
	if _, err := MessageToPoint(c, chunk); !errors.Is(err, ErrChunk) {
		t.Fatalf("получено %v", err)
	}
}
