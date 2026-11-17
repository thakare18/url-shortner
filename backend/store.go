package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type mongoStore struct {
	client       *mongo.Client
	links, codes *mongo.Collection
	cipher       *urlCipher
}

// Feature: only one reusable connection pool is created; requests share it with bounded deadlines.
func openMongo(ctx context.Context, c configuration) (*mongoStore, error) {
	crypt, err := newURLCipher(c.EncryptionKey, c.EncryptionKeyID)
	if err != nil {
		return nil, err
	}
	client, err := mongo.Connect(options.Client().ApplyURI(c.URI).SetAppName("QuickShortURL").SetMaxPoolSize(10).SetServerSelectionTimeout(8 * time.Second).SetConnectTimeout(8 * time.Second))
	if err != nil {
		return nil, err
	}
	store := &mongoStore{client: client, links: client.Database(c.Database).Collection("links"), codes: client.Database(c.Database).Collection("link_codes"), cipher: crypt}
	if err = client.Ping(ctx, readpref.Primary()); err == nil {
		err = store.setup(ctx)
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Disconnect(cleanup)
		return nil, err
	}
	return store, nil
}

func (s *mongoStore) setup(ctx context.Context) error {
	db := s.links.Database()
	for name, validator := range map[string]bson.M{"links": linkValidator(), "link_codes": codeValidator()} {
		err := db.CreateCollection(ctx, name, options.CreateCollection().SetValidator(validator).SetValidationLevel("strict").SetValidationAction("error"))
		var command mongo.CommandError
		if errors.As(err, &command) && command.Code == 48 {
			err = db.RunCommand(ctx, bson.D{{Key: "collMod", Value: name}, {Key: "validator", Value: validator}, {Key: "validationLevel", Value: "strict"}, {Key: "validationAction", Value: "error"}}).Err()
		}
		if err != nil {
			return err
		}
	}
	// Feature: a unique code index protects all replicas; the TTL index deletes only URL-bearing records.
	_, err := s.links.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "short_code", Value: 1}}, Options: options.Index().SetName("unique_short_code").SetUnique(true)},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetName("delete_expired_links").SetExpireAfterSeconds(0)},
		{Keys: bson.D{{Key: "created_at", Value: -1}}, Options: options.Index().SetName("created_at_desc")},
	})
	// link_codes._id is MongoDB's built-in unique index. Never TTL-delete these reservations.
	return err
}

// Feature: database validation enforces the documented schema, even for writes outside the API.
func linkValidator() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object", "additionalProperties": false,
		"required": []string{"_id", "short_code", "original_url_encrypted", "encryption_version", "encryption_key_id", "short_url", "is_custom", "created_at", "expiration_policy", "schema_version"},
		"oneOf": []bson.M{
			{"properties": bson.M{"expiration_policy": bson.M{"enum": []string{"never"}}}, "not": bson.M{"required": []string{"expires_at"}}},
			{"properties": bson.M{"expiration_policy": bson.M{"enum": []string{"1h", "24h", "7d", "30d"}}}, "required": []string{"expires_at"}},
		},
		"properties": bson.M{
			"_id":                    bson.M{"bsonType": "objectId"},
			"short_code":             bson.M{"bsonType": "string", "pattern": `^[a-zA-Z0-9][a-zA-Z0-9_-]{2,47}$`},
			"original_url_encrypted": bson.M{"bsonType": "binData"},
			"encryption_version":     bson.M{"enum": []string{encryptionVersion}},
			"encryption_key_id":      bson.M{"bsonType": "string", "pattern": `^[a-zA-Z0-9_-]{1,32}$`},
			"short_url":              bson.M{"bsonType": "string", "pattern": `^https?://`},
			"is_custom":              bson.M{"bsonType": "bool"}, "created_at": bson.M{"bsonType": "date"},
			"expires_at":        bson.M{"bsonType": "date"},
			"expiration_policy": bson.M{"enum": []string{"never", "1h", "24h", "7d", "30d"}},
			"schema_version":    bson.M{"bsonType": "int", "enum": []int{2}},
		},
	}}
}
func codeValidator() bson.M {
	return bson.M{"$jsonSchema": bson.M{"bsonType": "object", "additionalProperties": false, "required": []string{"_id"}, "properties": bson.M{
		"_id":        bson.M{"bsonType": "string", "pattern": `^[a-f0-9]{64}$`},
		"expires_at": bson.M{"bsonType": "date"},
	}}}
}

type codeReservation struct {
	CodeHash  string     `bson:"_id"`
	ExpiresAt *time.Time `bson:"expires_at,omitempty"`
}

// Feature: reservations retain a one-way code hash, not the original URL or the readable custom alias.
// Predictable aliases can still be guessed and hashed; this is minimization, not guaranteed anonymity.
func hashCode(code string) string {
	digest := sha256.Sum256([]byte(code))
	return hex.EncodeToString(digest[:])
}

func (s *mongoStore) Insert(ctx context.Context, link URL) error {
	// Encrypt before any database write; no plaintext URL is passed to the driver.
	record, err := s.cipher.encrypt(link)
	if err != nil {
		return err
	}
	// Feature: reserve first so a shared URL can never be reassigned, even after TTL removes its destination.
	// If a crash/timeout occurs between writes, the code stays reserved (safe failure), not reusable.
	_, err = s.codes.InsertOne(ctx, codeReservation{CodeHash: hashCode(link.ID), ExpiresAt: link.ExpiresAt})
	if mongo.IsDuplicateKeyError(err) {
		return errCodeTaken
	}
	if err != nil {
		return err
	}
	_, err = s.links.InsertOne(ctx, record)
	// Do not release on failure: an ambiguous write timeout might have persisted the URL.
	return err
}
func (s *mongoStore) Find(ctx context.Context, code string) (URL, error) {
	var record storedLink
	err := s.links.FindOne(ctx, bson.M{"short_code": code}).Decode(&record)
	if err == nil {
		if record.ExpiresAt != nil && !time.Now().Before(*record.ExpiresAt) {
			return URL{}, errExpired
		}
		return s.cipher.decrypt(record)
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return URL{}, err
	}
	// Feature: after the TTL deletion, the tiny reservation still allows the branded HTTP 410 page.
	var reservation codeReservation
	err = s.codes.FindOne(ctx, bson.M{"_id": hashCode(code)}).Decode(&reservation)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return URL{}, errNotFound
	}
	if err != nil {
		return URL{}, err
	}
	if reservation.ExpiresAt != nil && !time.Now().Before(*reservation.ExpiresAt) {
		return URL{}, errExpired
	}
	return URL{}, errNotFound
}
func (s *mongoStore) Exists(ctx context.Context, code string) (bool, error) {
	err := s.codes.FindOne(ctx, bson.M{"_id": hashCode(code)}, options.FindOne().SetProjection(bson.M{"_id": 1})).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, err
}
func (s *mongoStore) Ping(ctx context.Context) error { return s.client.Ping(ctx, readpref.Primary()) }
