package cipher

import (
	"encoding/hex"
	"errors"
	"fmt"

	"go-cipher/internal/crypto/gost"
)

type Gost struct{}

func NewGost() Gost {
	return Gost{}
}

func (Gost) Name() string {
	return "GOST 28147-89"
}

func (Gost) Methods() []string {
	return []string{"Simple", "Gamma", "Gamma with feedback", "Gamma with MAC"}
}

func (Gost) KeySize() int {
	return gost.KeySize
}

func (Gost) Run(method int, decrypt bool, in, key []byte) (string, error) {
	if len(key) != gost.KeySize {
		return "", fmt.Errorf("key must be %d bytes", gost.KeySize)
	}

	switch method {
	case 0:
		if decrypt {
			return string(gost.ECBDecrypt(in, key)), nil
		}
		return hex.EncodeToString(gost.ECBEncrypt(in, key)), nil
	case 1:
		if decrypt {
			if len(in) < 8 {
				return "", errors.New("ciphertext is too short")
			}
			return string(gost.GostGammaDecrypt(in[8:], key, in[:8])), nil
		}
		res, iv := gost.GostGammaEncrypt(in, key)
		return hex.EncodeToString(iv) + hex.EncodeToString(res), nil
	case 2:
		if decrypt {
			if len(in) < 8 {
				return "", errors.New("ciphertext is too short")
			}
			return string(gost.GammaWithFeedbackDecrypt(in[8:], key, in[:8])), nil
		}
		res, iv := gost.GammaWithFeedbackEncrypt(in, key)
		return hex.EncodeToString(iv) + hex.EncodeToString(res), nil
	case 3:
		if decrypt {
			if len(in) < 12 {
				return "", errors.New("ciphertext is too short (missing iv or MAC")
			}
			return string(gost.GammaDecryptWithMAC(in[12:], key, in[:8], in[8:12])), nil
		}
		res, iv, mac := gost.GammaEncryptWithMAC(in, key)
		return hex.EncodeToString(iv) + hex.EncodeToString(mac) + hex.EncodeToString(res), nil

	}
	return "", errors.New("unknown method")
}
