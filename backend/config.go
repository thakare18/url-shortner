package main

import (
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type configuration struct{ URI, Database, PublicURL, Address, EncryptionKey, EncryptionKeyID string }

var databasePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,63}$`)

func loadConfig() (configuration, error) {
	c := configuration{URI: os.Getenv("MONGODB_URI"), Database: env("MONGODB_DATABASE", "quickshorturl"), PublicURL: strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:3000/s"), "/"), Address: env("ADDR", "0.0.0.0:"+env("PORT", "8080"))}
	if !strings.HasPrefix(c.URI, "mongodb://") && !strings.HasPrefix(c.URI, "mongodb+srv://") {
		return c, errors.New("Set MONGODB_URI in backend/.env or the hosting environment; no JSON/memory fallback is enabled.")
	}
	if !databasePattern.MatchString(c.Database) {
		return c, errors.New("MONGODB_DATABASE must contain 1–63 letters, numbers, underscores or hyphens.")
	}
	public, err := url.Parse(c.PublicURL)
	if err != nil || public.Hostname() == "" || public.User != nil || public.RawQuery != "" || public.Fragment != "" || (public.Scheme != "https" && public.Scheme != "http") {
		return c, errors.New("PUBLIC_BASE_URL must be an absolute HTTP(S) URL without credentials, query or fragment.")
	}
	// Feature: production shared links must use HTTPS; localhost remains usable for development.
	local := public.Hostname() == "localhost" || public.Hostname() == "127.0.0.1" || public.Hostname() == "::1"
	if public.Scheme != "https" && !local {
		return c, errors.New("PUBLIC_BASE_URL must use HTTPS outside localhost.")
	}
	c.EncryptionKey = os.Getenv("URL_ENCRYPTION_KEY")
	c.EncryptionKeyID = env("URL_ENCRYPTION_KEY_ID", "v1")
	if _, err := newURLCipher(c.EncryptionKey, c.EncryptionKeyID); err != nil {
		return c, err
	}
	return c, nil
}
