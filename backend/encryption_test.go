package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func testEncryptionKey(t *testing.T) string {
	t.Helper()
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(key[:])
}
func TestURLCipher(t *testing.T) {
	key := testEncryptionKey(t)
	crypt, err := newURLCipher(key, "v1")
	if err != nil {
		t.Fatal(err)
	}
	link := URL{RecordID: bson.NewObjectID(), ID: "private", OriginalURL: "https://example.com/path?token=secret#fragment", CreationDate: time.Now().UTC(), ExpirationPolicy: "never", SchemaVersion: 2}
	first, err := crypt.encrypt(link)
	if err != nil {
		t.Fatal(err)
	}
	second, err := crypt.encrypt(link)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.EncryptedURL, second.EncryptedURL) {
		t.Fatal("nonces reused")
	}
	if first.OriginalURL != "" || first.DestinationHost != "" || first.DestinationScheme != "" {
		t.Fatal("plaintext kept in storage record")
	}
	restarted, err := newURLCipher(key, "v1")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restarted.decrypt(first)
	if err != nil || restored.OriginalURL != link.OriginalURL || restored.DestinationHost != "example.com" {
		t.Fatal("round trip failed")
	}
	wrongKey, err := newURLCipher(testEncryptionKey(t), "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = wrongKey.decrypt(first); err == nil {
		t.Fatal("wrong key accepted")
	}
	swapped := first
	swapped.ID = "different"
	if _, err = crypt.decrypt(swapped); err == nil {
		t.Fatal("swapped code accepted")
	}
	swapped = first
	swapped.RecordID = bson.NewObjectID()
	if _, err = crypt.decrypt(swapped); err == nil {
		t.Fatal("swapped record ID accepted")
	}
	swapped = first
	swapped.EncryptionKeyID = "v2"
	if _, err = crypt.decrypt(swapped); err == nil {
		t.Fatal("unknown key ID accepted")
	}
	tampered := first
	tampered.EncryptedURL = bytes.Clone(first.EncryptedURL)
	tampered.EncryptedURL[len(tampered.EncryptedURL)-1] ^= 1
	if _, err = crypt.decrypt(tampered); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	for _, encoded := range []string{"", "not-base64", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		if _, err = newURLCipher(encoded, "v1"); err == nil {
			t.Fatal("invalid key accepted")
		}
	}
}
