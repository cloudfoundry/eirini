package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Path: %s", html.EscapeString(r.URL.Path))
	})

	http.HandleFunc("/env", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, strings.Join(os.Environ(), "\n"))
	})

	http.HandleFunc("/exit", func(w http.ResponseWriter, _ *http.Request) {
		os.Exit(0)
	})

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
