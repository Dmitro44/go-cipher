package cipher

import (
	"fmt"

	"go-cipher/internal/crypto/mceliece"
)

type McEliece struct{}

func NewMcEliece() McEliece {
	return McEliece{}
}

func (McEliece) Name() string {
	return "McEliece"
}

// Открытый ключ — порождающая матрица G1 (k*n бит), закрытый — многочлен
// Гоппы, опорные элементы и S^{-1}. Размеры ключей на порядок больше
// модуля Рабина, поэтому панель показывает их placeholder'ом.
func (McEliece) KeyOptions() []KeyOption {
	// n = 1024, t = 38, k = 644 — параметры из методических указаний;
	// n = 256, t = 8, k = 192 — быстрый набор для демонстрации.
	return []KeyOption{
		{Label: "n=1024, t=38, k=644", Bits: 1024},
		{Label: "n=256, t=8, k=192", Bits: 256},
	}
}

func (McEliece) params(option KeyOption) (mceliece.Params, error) {
	switch option.Bits {
	case 1024:
		return mceliece.NewParams(10, 38)
	case 256:
		return mceliece.NewParams(8, 8)
	}
	return mceliece.Params{}, fmt.Errorf("%w: %d", mceliece.ErrUnsupported, option.Bits)
}

func (m McEliece) GenerateKeys(option KeyOption) (KeyPair, error) {
	params, err := m.params(option)
	if err != nil {
		return KeyPair{}, err
	}
	public, private, err := mceliece.GenerateKeys(params)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Public: public, Private: private}, nil
}

func (McEliece) Encrypt(pub, msg []byte) ([]byte, error) {
	key, err := mceliece.ParsePublicKey(pub)
	if err != nil {
		return nil, err
	}
	return key.Encrypt(msg)
}

func (McEliece) Decrypt(priv, cipherText []byte) ([]byte, error) {
	key, err := mceliece.ParsePrivateKey(priv)
	if err != nil {
		return nil, err
	}
	return key.Decrypt(cipherText)
}
