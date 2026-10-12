package gost

import "crypto/rand"

func GammaFeedbackGen(prev, key []byte) []byte {
	return BlockEncrypt32(prev, key)
}

func GammaWithFeedbackEncrypt(data, key []byte) ([]byte, []byte) {
	iv := make([]byte, 8)
	if _, err := rand.Read(iv); err != nil {
		panic(err)
	}

	padded := pad(data)
	cipherText := make([]byte, len(padded))
	prev := iv

	for i := 0; i < len(padded); i += 8 {
		gamma := GammaFeedbackGen(prev, key)

		block := padded[i : i+8]
		for j := range block {
			cipherText[i+j] = block[j] ^ gamma[j]
		}

		prev = cipherText[i : i+8]
	}

	return cipherText, iv
}

func GammaWithFeedbackDecrypt(data, key, iv []byte) []byte {
	result := make([]byte, len(data))
	prev := iv

	for i := 0; i < len(data); i += 8 {
		gamma := GammaFeedbackGen(prev, key)

		block := data[i : i+8]
		for j := range block {
			result[i+j] = block[j] ^ gamma[j]
		}

		prev = data[i : i+8]
	}

	return unpad(result)
}
