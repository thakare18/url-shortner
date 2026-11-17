package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Feature: BSON fields define the MongoDB record; existing JSON response fields stay compatible with Next.js.
// No IP address, user agent, cookies, referrer, location or user identity is collected.
type URL struct {
	RecordID          bson.ObjectID `json:"record_id" bson:"_id"`
	ID                string        `json:"id" bson:"short_code"`
	OriginalURL       string        `json:"original_url" bson:"-"`
	ShortURL          string        `json:"short_url" bson:"short_url"`
	DestinationHost   string        `json:"destination_host" bson:"-"`
	DestinationScheme string        `json:"destination_scheme" bson:"-"`
	IsCustom          bool          `json:"is_custom" bson:"is_custom"`
	CreationDate      time.Time     `json:"creation_date" bson:"created_at"`
	ExpiresAt         *time.Time    `json:"expires_at" bson:"expires_at,omitempty"`
	ExpirationPolicy  string        `json:"expiration_policy" bson:"expiration_policy"`
	SchemaVersion     int           `json:"schema_version" bson:"schema_version"`
}

// Feature: handlers use a storage interface. Production only uses MongoDB; tests use an isolated fake.
type linkStore interface {
	Insert(context.Context, URL) error
	Find(context.Context, string) (URL, error)
	Exists(context.Context, string) (bool, error)
	Ping(context.Context) error
}

type server struct {
	store     linkStore
	publicURL string
}

var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,47}$`)
var (
	errNotFound  = errors.New("link not found")
	errExpired   = errors.New("link expired")
	errCodeTaken = errors.New("short code taken")
)

// Feature: the expired-link screen ships inside the Go binary.
//
//go:embed expired.html
var expiredPage string

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func apiError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Feature: availability is advisory; the database unique key decides the winner during concurrent creation.
func (s *server) availability(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	if !slugPattern.MatchString(slug) {
		apiError(w, 400, "Use 3–48 letters, numbers, hyphens or underscores; start with a letter or number.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	exists, err := s.store.Exists(ctx, slug)
	if err != nil {
		apiError(w, 503, "Link storage is temporarily unavailable. Please retry.")
		return
	}
	writeJSON(w, 200, map[string]bool{"available": !exists})
}

func (s *server) ShortURLHandler(w http.ResponseWriter, r *http.Request) {
	var data struct {
		URL         string `json:"url"`
		CustomAlias string `json:"custom_alias"`
		ExpiresIn   string `json:"expires_in"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&data); err != nil {
		apiError(w, 400, "Invalid request body.")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		apiError(w, 400, "Send one JSON object.")
		return
	}
	data.URL = strings.TrimSpace(data.URL)
	parsed, err := url.Parse(data.URL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || len(data.URL) > 8192 {
		apiError(w, 400, "Enter a valid http:// or https:// URL without embedded credentials.")
		return
	}
	if data.CustomAlias != "" && !slugPattern.MatchString(data.CustomAlias) {
		apiError(w, 400, "Custom ending must be 3–48 letters, numbers, hyphens or underscores.")
		return
	}
	if data.ExpiresIn == "" {
		data.ExpiresIn = "never"
	}
	var expiresAt *time.Time
	now := time.Now().UTC().Truncate(time.Millisecond)
	if data.ExpiresIn != "never" {
		durations := map[string]time.Duration{"1h": time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}
		duration, ok := durations[data.ExpiresIn]
		if !ok {
			apiError(w, 400, "Choose a supported expiration.")
			return
		}
		expiry := now.Add(duration)
		expiresAt = &expiry
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	// Feature: random-code collisions retry; custom collisions return 409 without replacing anyone's link.
	for attempt := 0; attempt < 5; attempt++ {
		id := data.CustomAlias
		if id == "" {
			var random [6]byte
			if _, err := rand.Read(random[:]); err != nil {
				apiError(w, 500, "Could not generate a link.")
				return
			}
			id = hex.EncodeToString(random[:])
		}
		link := URL{RecordID: bson.NewObjectID(), ID: id, OriginalURL: data.URL, ShortURL: s.publicURL + "/" + id,
			DestinationHost: strings.ToLower(parsed.Hostname()), DestinationScheme: parsed.Scheme, IsCustom: data.CustomAlias != "",
			CreationDate: now, ExpiresAt: expiresAt, ExpirationPolicy: data.ExpiresIn, SchemaVersion: 2}
		err = s.store.Insert(ctx, link)
		if errors.Is(err, errCodeTaken) {
			if data.CustomAlias != "" {
				apiError(w, 409, "That ending is already taken. Try another.")
				return
			}
			continue
		}
		if err != nil {
			apiError(w, 503, "Could not save your link. Storage is temporarily unavailable.")
			return
		}
		writeJSON(w, http.StatusCreated, link)
		return
	}
	apiError(w, 503, "Could not allocate a short link. Please retry.")
}

func (s *server) RedirectHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	link, err := s.store.Find(ctx, r.PathValue("id"))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	// Feature: expiry is enforced immediately, independently of MongoDB's asynchronous TTL cleanup.
	if errors.Is(err, errExpired) || (err == nil && link.ExpiresAt != nil && !time.Now().Before(*link.ExpiresAt)) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(expiredPage))
		return
	}
	if errors.Is(err, errNotFound) {
		http.Error(w, "Link not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Link storage is temporarily unavailable. Please retry.", 503)
		return
	}
	http.Redirect(w, r, link.OriginalURL, http.StatusFound)
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shorten", s.ShortURLHandler)
	mux.HandleFunc("POST /shorten", s.ShortURLHandler)
	mux.HandleFunc("GET /api/availability", s.availability)
	mux.HandleFunc("GET /redirect/{id}", s.RedirectHandler)
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"public_base_url": s.publicURL})
	})
	// Feature: lightweight liveness for Render/Actions; readiness checks the database separately.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.store.Ping(ctx); err != nil {
			apiError(w, 503, "Database unavailable.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	return mux
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	migrate := flag.Bool("migrate-plaintext", false, "Encrypt legacy MongoDB URL records, then exit")
	flag.Parse()
	// Feature: .env works locally; existing Render environment variables always take precedence.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal("Could not read .env file.")
	}
	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	store, err := openMongo(ctx, config)
	cancel()
	// Do not log driver errors here: connection strings may contain credentials.
	if err != nil {
		log.Fatal("MongoDB startup failed. Check credentials, Atlas network access, database permissions and connectivity.")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = store.client.Disconnect(ctx)
	}()
	if *migrate {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		count, err := store.migratePlaintext(ctx)
		if err != nil {
			log.Fatal("Encryption migration failed. No URL contents were logged; check schema/key configuration and retry.")
		}
		log.Printf("Encrypted %d legacy link records. Provider backups and old JSON files require separate retention handling.", count)
		return
	}
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	legacy, err := store.links.CountDocuments(ctx, bson.M{"original_url": bson.M{"$exists": true}})
	cancel()
	if err != nil || legacy > 0 {
		log.Fatal("Storage verification failed or plaintext legacy links exist. Run go run . -migrate-plaintext before starting the server.")
	}
	s := &server{store: store, publicURL: config.PublicURL}
	httpServer := &http.Server{Addr: config.Address, Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopped)
	go func() {
		<-stopped
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()
	log.Printf("QuickShortURL API listening on %s (MongoDB connected)", config.Address)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("HTTP server stopped unexpectedly.")
	}
}
