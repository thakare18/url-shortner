// Run locally: go run ./cmd/keygen. Save the output in URL_ENCRYPTION_KEY, never in Git.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
)

func main() {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		log.Fatal("Could not generate an encryption key.")
	}
	fmt.Println(base64.StdEncoding.EncodeToString(key[:]))
}
