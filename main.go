package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// struct to represent a URL
type URL struct {
	ID           string `json:"id"`
	OriginalURL  string `json:"original_url"`
	ShortURL     string `json:"short_url"`
	CreationDate string `json:"creation_date"`
}

/* mapping in short url that all structure will be mapped
d92sa3 ---> {
ID: "d92sa3",
OriginalURL: "https://www.google.com",
ShortURL: "https://short.url/d92sa3",
CreationDate: time.Now()
}
*/

// creating a in memory database to store the urls
var urlDB = make(map[string]URL)

// function to generate a short url
func generateShortURL(originalURL string) string {
	var hasher = md5.New()
	hasher.Write([]byte(originalURL))
	fmt.Println("Hasher", hasher)
	data := hasher.Sum(nil)
	fmt.Println("Data", data)
	hash := hex.EncodeToString(data)
	fmt.Println("Hash", hash)
	fmt.Println("hash final string", hash[:8])
	return hash[:8]
}

// function to create a short url and store it into an database
func createURL(originalURL string) string {
	shortURL := generateShortURL(originalURL)
	id := shortURL // gereating id from short url
	urlDB[id] = URL{
		ID:           id,
		OriginalURL:  originalURL,
		ShortURL:     shortURL,
		CreationDate: time.Now().Format(time.RFC3339),
	}
	return shortURL
}

func getURL(id string) (URL, error) {
	url, ok := urlDB[id]
	if !ok {
		return URL{}, errors.New("URL not found")
	}
	return url, nil
}

func RootPageURL(w http.ResponseWriter, r *http.Request) { // w is a response writer and r is a request
	fmt.Println("GET method called")
	fmt.Fprintf(w, "Hello World") // fprintf is used to write a the reponse to the client
}

// handler function to handle the short url requests
func ShortURLHandler(w http.ResponseWriter, r *http.Request) {
	var data struct {
		URL string `json:"url"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	shortURL_ := createURL(data.URL)
	// fmt.Fprintf(w, shortURL)
	response := struct {
		ShortURL string `json:"short_url"`
	}{ShortURL: shortURL_}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handler function to handle the redirect requests
func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/redirect/"):]

	url, err := getURL(id)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusNotFound)
	}
	http.Redirect(w, r, url.OriginalURL, http.StatusFound)
}

func main() {
	// fmt.Println("Starting a url shortner...")
	// OrignalURL := "https://github.com/thakare18"
	// generateShortURL(OrignalURL)

	// register the handler function to handle all requests to rool url path "/"
	http.HandleFunc("/", RootPageURL)
	http.HandleFunc("/shorten", ShortURLHandler)
	http.HandleFunc("/redirect/", RedirectHandler)

	// starting a http server on port 3000
	fmt.Println("starting a server on port 3000...")
	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		fmt.Println("Error while starting the server:", err)
	}

}
