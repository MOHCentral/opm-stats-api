package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	token := "8b6a271b-eed5-4e55-995a-d5494f3ca94b"
	fmt.Println("Hash:", hashToken(token))
}
