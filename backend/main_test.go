package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Test-only fake: production has no in-memory or JSON fallback.
type memoryStore struct {
	mu          sync.Mutex
	links       map[string]URL
	codes       map[string]*time.Time
	unavailable bool
}

func newMemoryStore() *memoryStore {
	return &memoryStore{links: map[string]URL{}, codes: map[string]*time.Time{}}
}
func (s *memoryStore) Insert(_ context.Context, link URL) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return errors.New("offline")
	}
	if _, ok := s.codes[link.ID]; ok {
		return errCodeTaken
	}
	s.codes[link.ID] = link.ExpiresAt
	s.links[link.ID] = link
	return nil
}
func (s *memoryStore) Find(_ context.Context, code string) (URL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return URL{}, errors.New("offline")
	}
	if link, ok := s.links[code]; ok {
		return link, nil
	}
	if expiry, ok := s.codes[code]; ok && expiry != nil && !time.Now().Before(*expiry) {
		return URL{}, errExpired
	}
	return URL{}, errNotFound
}
func (s *memoryStore) Exists(_ context.Context, code string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return false, errors.New("offline")
	}
	_, ok := s.codes[code]
	return ok, nil
}
func (s *memoryStore) Ping(_ context.Context) error {
	if s.unavailable {
		return errors.New("offline")
	}
	return nil
}
func testServer(t *testing.T) *server {
	t.Helper()
	return &server{store: newMemoryStore(), publicURL: "http://localhost:3000/s"}
}
func request(s *server, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestCreateMetadataAndRedirect(t *testing.T) {
	s := testServer(t)
	w := request(s, "POST", "/api/shorten", `{"url":"https://example.com/a?b=c","custom_alias":"my-portfolio","expires_in":"24h"}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var link URL
	if err := json.Unmarshal(w.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if link.RecordID.IsZero() || link.ID != "my-portfolio" || link.ShortURL != "http://localhost:3000/s/my-portfolio" || link.ExpiresAt == nil || time.Until(*link.ExpiresAt) < 23*time.Hour || !link.IsCustom || link.DestinationHost != "example.com" || link.DestinationScheme != "https" || link.SchemaVersion != 2 || link.ExpirationPolicy != "24h" {
		t.Fatalf("unexpected metadata: %+v", link)
	}
	w = request(s, "GET", "/redirect/my-portfolio", "")
	if w.Code != 302 || w.Header().Get("Location") != link.OriginalURL {
		t.Fatal("redirect failed")
	}
	w = request(s, "GET", "/api/availability?slug=my-portfolio", "")
	if !strings.Contains(w.Body.String(), `"available":false`) {
		t.Fatal(w.Body)
	}
}
func TestValidation(t *testing.T) {
	s := testServer(t)
	for _, body := range []string{`{"url":"javascript:alert(1)"}`, `{"url":"https://"}`, `{"url":"https://user:pass@example.com"}`, `{"url":"https://example.com","custom_alias":"a/b"}`, `{"url":"https://example.com","expires_in":"invalid"}`, `{"url":"https://example.com"} {}`, `not json`} {
		if w := request(s, "POST", "/api/shorten", body); w.Code != 400 {
			t.Errorf("accepted %s: %d", body, w.Code)
		}
	}
	if w := request(s, "GET", "/api/shorten", ""); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if w := request(s, "GET", "/redirect/missing", ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestExpiryBeforeAndAfterCleanup(t *testing.T) {
	s := testServer(t)
	store := s.store.(*memoryStore)
	past := time.Now().Add(-time.Minute)
	_ = store.Insert(context.Background(), URL{ID: "expired", OriginalURL: "https://example.com", ExpiresAt: &past})
	for _, afterCleanup := range []bool{false, true} {
		if afterCleanup {
			delete(store.links, "expired")
		}
		w := request(s, "GET", "/redirect/expired", "")
		if w.Code != 410 || w.Header().Get("Location") != "" || !strings.Contains(w.Body.String(), "This link has expired.") || !strings.Contains(w.Body.String(), `href="/#destination"`) {
			t.Fatalf("expiry: %d %s", w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("expiry cached")
		}
		w = request(s, "POST", "/api/shorten", `{"url":"https://example.org","custom_alias":"expired"}`)
		if w.Code != 409 {
			t.Fatal(w.Code)
		}
	}
}
func TestConcurrentCustomAlias(t *testing.T) {
	s := testServer(t)
	statuses := make(chan int, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- request(s, "POST", "/api/shorten", `{"url":"https://example.com","custom_alias":"only-one"}`).Code
		}()
	}
	wg.Wait()
	close(statuses)
	created := 0
	for code := range statuses {
		if code == 201 {
			created++
		} else if code != 409 {
			t.Fatal(code)
		}
	}
	if created != 1 {
		t.Fatal("duplicate claims")
	}
}
func TestRandomLinksAndUnavailableStorage(t *testing.T) {
	s := testServer(t)
	for i := 0; i < 2; i++ {
		w := request(s, "POST", "/api/shorten", `{"url":"https://example.com"}`)
		if w.Code != 201 {
			t.Fatal(w.Body)
		}
	}
	store := s.store.(*memoryStore)
	if len(store.links) != 2 {
		t.Fatal("collision")
	}
	for _, link := range store.links {
		if link.IsCustom || link.ExpiresAt != nil || link.ExpirationPolicy != "never" {
			t.Fatal("random metadata")
		}
	}
	store.unavailable = true
	for _, call := range []struct{ method, path, body string }{{"POST", "/api/shorten", `{"url":"https://example.com"}`}, {"GET", "/api/availability?slug=abc", ""}, {"GET", "/redirect/abc", ""}, {"GET", "/api/ready", ""}} {
		if w := request(s, call.method, call.path, call.body); w.Code != 503 {
			t.Fatal(w.Code)
		}
	}
	if w := request(s, "GET", "/api/health", ""); w.Code != 200 {
		t.Fatal("liveness must not query database")
	}
}
func TestPublicConfig(t *testing.T) {
	s := testServer(t)
	s.publicURL = "https://links.example.com/s"
	w := request(s, "GET", "/api/config", "")
	if !strings.Contains(w.Body.String(), s.publicURL) {
		t.Fatal(w.Body)
	}
}
func TestBSONContainsOnlyLinkMetadata(t *testing.T) {
	data, err := bson.Marshal(URL{RecordID: bson.NewObjectID(), ID: "abc", ExpirationPolicy: "never", SchemaVersion: 2})
	if err != nil {
		t.Fatal(err)
	}
	var record bson.M
	if err = bson.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if _, ok := record["expires_at"]; ok {
		t.Fatal("never-expiring links must omit the TTL field")
	}
	expected := map[string]bool{"_id": true, "short_code": true, "short_url": true, "is_custom": true, "created_at": true, "expiration_policy": true, "schema_version": true}
	for key := range record {
		if !expected[key] {
			t.Fatalf("unexpected metadata: %s", key)
		}
	}
}
func TestConfiguration(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("missing URI accepted")
	}
	t.Setenv("MONGODB_URI", "mongodb+srv://unit-test.invalid")
	t.Setenv("PORT", "10000")
	t.Setenv("ADDR", "")
	t.Setenv("PUBLIC_BASE_URL", "https://quickshorturl.example/s/")
	t.Setenv("MONGODB_DATABASE", "quickshorturl")
	t.Setenv("URL_ENCRYPTION_KEY", testEncryptionKey(t))
	t.Setenv("URL_ENCRYPTION_KEY_ID", "v1")
	c, err := loadConfig()
	if err != nil || c.Address != "0.0.0.0:10000" || c.PublicURL != "https://quickshorturl.example/s" {
		t.Fatalf("config: %+v %v", c, err)
	}
	t.Setenv("PUBLIC_BASE_URL", "http://public.example/s")
	if _, err := loadConfig(); err == nil {
		t.Fatal("insecure public URL accepted")
	}
}
