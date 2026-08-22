package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const Header = "HashSHA256"

func Sum(key string, body []byte) string {
	return hex.EncodeToString(mac(key, body))
}

func Equal(key string, body []byte, got string) bool {
	want, err := hex.DecodeString(got)
	if err != nil {
		return false
	}

	return hmac.Equal(mac(key, body), want)
}

func mac(key string, body []byte) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(body)
	return h.Sum(nil)
}
