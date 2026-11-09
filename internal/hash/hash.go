package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	stdhash "hash"
	"sync"
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

func newMAC(key string) stdhash.Hash {
	return hmac.New(sha256.New, []byte(key))
}

func sumWith(macs *sync.Pool, body []byte) string {
	var scratch [sha256.Size]byte

	h := macs.Get().(stdhash.Hash)
	h.Reset()
	h.Write(body)
	digest := h.Sum(scratch[:0])
	macs.Put(h)

	return hex.EncodeToString(digest)
}

func equalWith(macs *sync.Pool, body []byte, got string) bool {
	if len(got) != hex.EncodedLen(sha256.Size) {
		return false
	}

	var want [sha256.Size]byte
	if _, err := hex.Decode(want[:], []byte(got)); err != nil {
		return false
	}

	var scratch [sha256.Size]byte

	h := macs.Get().(stdhash.Hash)
	h.Reset()
	h.Write(body)
	digest := h.Sum(scratch[:0])
	macs.Put(h)

	return hmac.Equal(digest, want[:])
}
