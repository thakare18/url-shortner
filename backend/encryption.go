package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const encryptionVersion = "AES-256-GCM/v1"

type urlCipher struct {
	aead  cipher.AEAD
	keyID string
}

// Feature: a real random 256-bit key is required; Base64 is only the key's transport format.
// Go's AEAD generates and prepends a fresh nonce and authenticates every encrypted destination.
func newURLCipher(encodedKey, keyID string) (*urlCipher, error) {
	key, err := base64.StdEncoding.Strict().DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("Set URL_ENCRYPTION_KEY to a Base64-encoded 32-byte random key; run go run ./cmd/keygen locally.")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`).MatchString(keyID) {
		return nil, errors.New("URL_ENCRYPTION_KEY_ID must be 1–32 letters, numbers, underscores or hyphens.")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("Could not initialize URL encryption.")
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, errors.New("Could not initialize URL encryption.")
	}
	return &urlCipher{aead: aead, keyID: keyID}, nil
}

// Plaintext API-only fields have bson:"-"; only this encrypted storage shape can pass the DB validator.
type storedLink struct {
	URL               `bson:",inline"`
	EncryptedURL      []byte `bson:"original_url_encrypted"`
	EncryptionVersion string `bson:"encryption_version"`
	EncryptionKeyID   string `bson:"encryption_key_id"`
}

func encryptionContext(link URL, keyID string) []byte {
	return []byte("QuickShortURL:url:v1\x00" + keyID + "\x00" + link.RecordID.Hex() + "\x00" + link.ID)
}
func (c *urlCipher) encrypt(link URL) (storedLink, error) {
	if link.OriginalURL == "" || link.RecordID.IsZero() || !slugPattern.MatchString(link.ID) {
		return storedLink{}, errors.New("Invalid link encryption input.")
	}
	link.SchemaVersion = 2
	encrypted := c.aead.Seal(nil, nil, []byte(link.OriginalURL), encryptionContext(link, c.keyID))
	// Feature: clear the plaintext from the DB document even before BSON serialization.
	link.OriginalURL = ""
	link.DestinationHost = ""
	link.DestinationScheme = ""
	return storedLink{URL: link, EncryptedURL: encrypted, EncryptionVersion: encryptionVersion, EncryptionKeyID: c.keyID}, nil
}
func (c *urlCipher) decrypt(record storedLink) (URL, error) {
	if record.EncryptionVersion != encryptionVersion || record.EncryptionKeyID != c.keyID || record.SchemaVersion != 2 {
		return URL{}, errors.New("Unsupported encrypted link.")
	}
	plaintext, err := c.aead.Open(nil, nil, record.EncryptedURL, encryptionContext(record.URL, c.keyID))
	if err != nil {
		return URL{}, errors.New("Could not decrypt link.")
	}
	destination := string(plaintext)
	parsed, err := url.Parse(destination)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return URL{}, errors.New("Invalid decrypted link.")
	}
	link := record.URL
	link.OriginalURL = destination
	link.DestinationHost = strings.ToLower(parsed.Hostname())
	link.DestinationScheme = parsed.Scheme
	return link, nil
}

// Feature: explicit migration encrypts legacy MongoDB documents in place, removing all plaintext destination fields.
// A conditional update makes interrupted runs safely repeatable. Existing files/backups are not silently deleted.
func (s *mongoStore) migratePlaintext(ctx context.Context) (int64, error) {
	cursor, err := s.links.Find(ctx, bson.M{"original_url": bson.M{"$exists": true}})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var count int64
	for cursor.Next(ctx) {
		var legacy struct {
			URL       `bson:",inline"`
			Plaintext string `bson:"original_url"`
		}
		if err := cursor.Decode(&legacy); err != nil {
			return count, err
		}
		legacy.URL.OriginalURL = legacy.Plaintext
		record, err := s.cipher.encrypt(legacy.URL)
		if err != nil {
			return count, err
		}
		result, err := s.links.UpdateOne(ctx, bson.M{"_id": legacy.RecordID, "original_url": bson.M{"$exists": true}}, bson.M{
			"$set":   bson.M{"original_url_encrypted": record.EncryptedURL, "encryption_version": record.EncryptionVersion, "encryption_key_id": record.EncryptionKeyID, "schema_version": 2},
			"$unset": bson.M{"original_url": "", "destination_host": "", "destination_scheme": ""},
		})
		if err != nil {
			return count, err
		}
		count += result.ModifiedCount
	}
	return count, cursor.Err()
}
