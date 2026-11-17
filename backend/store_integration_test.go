package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Feature: real database tests are opt-in and use a unique disposable database, never the configured app database.
func TestMongoIntegration(t *testing.T) {
	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Skip("set MONGODB_TEST_URI to a disposable MongoDB instance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	c := configuration{URI: uri, Database: "quickshorturl_test_" + bson.NewObjectID().Hex(), EncryptionKey: testEncryptionKey(t), EncryptionKeyID: "v1"}
	store, err := openMongo(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = store.links.Database().Drop(cleanup)
		_ = store.client.Disconnect(cleanup)
	}()
	s := &server{store: store, publicURL: "https://quickshorturl.example/s"}
	w := request(s, "POST", "/api/shorten", `{"url":"https://example.com/private?token=not-logged","custom_alias":"persistent","expires_in":"1h"}`)
	if w.Code != 201 {
		t.Fatalf("insert: %d %s", w.Code, w.Body)
	}
	var link URL
	_ = json.Unmarshal(w.Body.Bytes(), &link)
	var raw bson.Raw
	if err := store.links.FindOne(ctx, bson.M{"short_code": "persistent"}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"original_url", "destination_host", "destination_scheme"} {
		if _, err := raw.LookupErr(forbidden); err == nil {
			t.Fatal("plaintext destination field in database")
		}
	}
	if bytes.Contains(raw, []byte("example.com")) || bytes.Contains(raw, []byte("not-logged")) {
		t.Fatal("plaintext destination leaked into BSON")
	}
	// A separate client simulates a new backend instance after redeploy.
	second, err := openMongo(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer second.client.Disconnect(context.Background())
	saved, err := second.Find(ctx, "persistent")
	if err != nil || saved.OriginalURL != link.OriginalURL || saved.RecordID != link.RecordID {
		t.Fatalf("persistence: %+v %v", saved, err)
	}
	statuses := make(chan int, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- request(s, "POST", "/api/shorten", `{"url":"https://example.com","custom_alias":"racing"}`).Code
		}()
	}
	wg.Wait()
	close(statuses)
	created := 0
	for code := range statuses {
		if code == 201 {
			created++
		} else if code != 409 {
			t.Fatalf("race status: %d", code)
		}
	}
	if created != 1 {
		t.Fatal("duplicate custom code")
	}
	// The validator rejects undeclared visitor metadata.
	doc := bson.M{"_id": bson.NewObjectID(), "ip_address": "127.0.0.1"}
	if _, err := store.links.InsertOne(ctx, doc); err == nil {
		t.Fatal("invalid schema accepted")
	}
	missingExpiry, err := store.cipher.encrypt(link)
	if err != nil {
		t.Fatal(err)
	}
	missingExpiry.RecordID = bson.NewObjectID()
	missingExpiry.ID = "invalid-expiry"
	missingExpiry.ExpiresAt = nil
	if _, err := store.links.InsertOne(ctx, missingExpiry); err == nil {
		t.Fatal("expiring policy without expires_at accepted")
	}
	indexes, err := store.links.Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var specs []bson.M
	if err = indexes.All(ctx, &specs); err != nil {
		t.Fatal(err)
	}
	ttl, unique := false, false
	for _, spec := range specs {
		if spec["name"] == "delete_expired_links" {
			ttl = spec["expireAfterSeconds"] == int32(0) || spec["expireAfterSeconds"] == int64(0)
		}
		if spec["name"] == "unique_short_code" {
			unique = spec["unique"] == true
		}
	}
	if !ttl || !unique {
		t.Fatalf("indexes: %+v", specs)
	}
	past := time.Now().Add(-time.Minute).UTC()
	expired := link
	expired.RecordID = bson.NewObjectID()
	expired.ID = "ttl-expired"
	expired.ExpiresAt = &past
	if err = store.Insert(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if w = request(s, "GET", "/redirect/ttl-expired", ""); w.Code != 410 {
		t.Fatal("expired link redirected before TTL cleanup")
	}
	if os.Getenv("MONGODB_TEST_TTL") == "1" {
		// Local test container's TTL monitor runs every second; no server setting is changed by this test.
		deadline := time.Now().Add(12 * time.Second)
		for {
			err = store.links.FindOne(ctx, bson.M{"short_code": "ttl-expired"}).Err()
			if errors.Is(err, mongo.ErrNoDocuments) {
				break
			}
			if err != nil || time.Now().After(deadline) {
				t.Fatalf("TTL did not delete expired destination: %v", err)
			}
			time.Sleep(200 * time.Millisecond)
		}
	} else {
		_, err = store.links.DeleteOne(ctx, bson.M{"short_code": "ttl-expired"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if w = request(s, "GET", "/redirect/ttl-expired", ""); w.Code != 410 {
		t.Fatal("expired page lost after TTL cleanup")
	}
	if exists, err := store.Exists(ctx, "ttl-expired"); err != nil || !exists {
		t.Fatal("expired code became reusable")
	}
	var reservation bson.M
	if err := store.codes.FindOne(ctx, bson.M{"_id": hashCode("ttl-expired")}).Decode(&reservation); err != nil {
		t.Fatal(err)
	}
	if len(reservation) != 2 || reservation["_id"] == "ttl-expired" {
		t.Fatal("expired reservation must retain only code hash and expiry")
	}
	// Expiring links are removed; a never-expiring link omits expires_at and is not eligible for TTL.
	if w = request(s, "POST", "/api/shorten", `{"url":"https://example.com","custom_alias":"lasting"}`); w.Code != 201 {
		t.Fatal(w.Body)
	}
	count, err := store.links.CountDocuments(ctx, bson.M{"short_code": "lasting", "expires_at": bson.M{"$exists": false}})
	if err != nil || count != 1 {
		t.Fatal("never-expiring TTL behavior")
	}
	// A failure after reserving a code must not make that code reusable.
	failed := link
	failed.RecordID = link.RecordID
	failed.ID = "failed-write"
	if err = store.Insert(ctx, failed); err == nil {
		t.Fatal("expected duplicate record ID error")
	}
	if exists, err := store.Exists(ctx, "failed-write"); err != nil || !exists {
		t.Fatal("ambiguous failed write released code")
	}
	if _, err = store.Find(ctx, "failed-write"); !errors.Is(err, errNotFound) {
		t.Fatal("failed insert created a readable URL")
	}
	// Explicitly test the unique link index even if a writer bypasses the reservation code.
	duplicate := link
	duplicate.RecordID = bson.NewObjectID()
	duplicateRecord, err := store.cipher.encrypt(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.links.InsertOne(ctx, duplicateRecord, options.InsertOne()); !mongo.IsDuplicateKeyError(err) {
		t.Fatal("unique short-code index did not enforce uniqueness")
	}
	// A tampered ciphertext must fail closed and must never redirect.
	if _, err := store.links.UpdateOne(ctx, bson.M{"short_code": "persistent"}, bson.M{"$set": bson.M{"original_url_encrypted": []byte("tampered")}}); err != nil {
		t.Fatal(err)
	}
	if w := request(s, "GET", "/redirect/persistent", ""); w.Code != 503 || w.Header().Get("Location") != "" {
		t.Fatal("tampered ciphertext redirected")
	}
	// Seed a legacy record only for migration testing, explicitly bypassing the new validator.
	legacyID := bson.NewObjectID()
	legacy := bson.M{"_id": legacyID, "short_code": "legacy", "original_url": "https://legacy.example/private", "short_url": "https://quickshorturl.example/s/legacy", "destination_host": "legacy.example", "destination_scheme": "https", "is_custom": true, "created_at": time.Now().UTC(), "expiration_policy": "never", "schema_version": 1}
	if _, err := store.links.InsertOne(ctx, legacy, options.InsertOne().SetBypassDocumentValidation(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.codes.InsertOne(ctx, codeReservation{CodeHash: hashCode("legacy")}); err != nil {
		t.Fatal(err)
	}
	countMigrated, err := store.migratePlaintext(ctx)
	if err != nil || countMigrated != 1 {
		t.Fatalf("migration: %d %v", countMigrated, err)
	}
	restored, err := store.Find(ctx, "legacy")
	if err != nil || restored.OriginalURL != "https://legacy.example/private" {
		t.Fatal("migration lost destination")
	}
	if err := store.links.FindOne(ctx, bson.M{"_id": legacyID}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("legacy.example")) {
		t.Fatal("migration kept plaintext")
	}
	if count, err := store.migratePlaintext(ctx); err != nil || count != 0 {
		t.Fatal("migration not idempotent")
	}
}
