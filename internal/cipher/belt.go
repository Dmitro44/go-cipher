package cipher

import (
	"encoding/hex"
	"errors"
	"fmt"

	"go-cipher/internal/crypto/belt"
)

type Belt struct{}

func NewBelt() Belt {
	return Belt{}
}

func (Belt) Name() string {
	return "STB 34.101.31-2011"
}

func (Belt) Methods() []string {
	return []string{"Simple", "Gamma with Feedback"}
}

func (Belt) KeySize() int {
	return belt.KeySize
}

func (Belt) Run(method int, decrypt bool, in, key []byte) (string, error) {
	if len(key) != belt.KeySize {
		return "", fmt.Errorf("key must be %d bytes", belt.KeySize)
	}

	switch method {
	case 0:
		if decrypt {
			res, _ := belt.ECBDecrypt(in, key)
			return string(res), nil
		}
		res, _ := belt.ECBEncrypt(in, key)
		return hex.EncodeToString(res), nil
	case 1:
		if decrypt {
			if len(in) < 16 {
				return "", errors.New("ciphertext is too short")
			}
			res, _ := belt.GammaWithFeedbackDecrypt(in[16:], key, in[:16])
			return string(res), nil
		}
		res, sync, _ := belt.GammaWithFeedbackEncrypt(in, key)
		return hex.EncodeToString(sync) + hex.EncodeToString(res), nil
	}

	return "", errors.New("unknown method")
}
