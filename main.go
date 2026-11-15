package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
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
var usrDB = make(map[string]URL)

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

func main() {
	fmt.Println("Starting a url shortner...")
	OrignalURL := "https://github.com/thakare18"
	generateShortURL(OrignalURL)
}
