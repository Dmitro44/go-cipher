package gost

// for ecb and gammaWithFeedback
func pad(data []byte) []byte {
	padLen := BlockSize - len(data)%BlockSize

	padded := make([]byte, len(data)+padLen)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	return padded
}

func unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	padLen := int(data[len(data)-1])
	if padLen < 1 || padLen > BlockSize || padLen > len(data) {
		return data
	}

	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return data
		}
	}

	return data[:len(data)-padLen]
}

// for mac gen
func zeroPad(data []byte) []byte {
	rem := len(data) % BlockSize
	if rem == 0 {
		return data
	}

	padded := make([]byte, len(data)+BlockSize-rem)
	copy(padded, data)

	return padded
}
