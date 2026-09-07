package service

import (
	"crypto/rand"
	"math/big"
)

const (
	CodeLength  = 6
	alphabet    = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	maxAttempts = 3
)

var reservedCodes = map[string]bool{
	"health": true, "status": true, "static": true, "assets": true, "public": true,
	"robots": true, "search": true, "config": true, "signup": true, "logout": true,
}

func generateCode() (string, error) {
	for {
		b := make([]byte, CodeLength)
		for i := range b {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				return "", err
			}
			b[i] = alphabet[n.Int64()]
		}
		code := string(b)
		if !reservedCodes[code] {
			return code, nil
		}
	}
}

func IsWellFormedCode(s string) bool {
	if len(s) != CodeLength {
		return false
	}
	for i := range len(s) {
		if !isBase62(s[i]) {
			return false
		}
	}
	return true
}

func isBase62(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
