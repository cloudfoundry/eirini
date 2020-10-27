package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", HelloHandler)
	http.HandleFunc("/exit", ExitHandler)

	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("an unexpected error occurred: %w", err))
		os.Exit(1)
	}
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hi, I'm not Dora!")
}

func ExitHandler(w http.ResponseWriter, r *http.Request) {
	exitCodes, ok := r.URL.Query()["exitCode"]
	if !ok {
		os.Exit(0)
	}

	if len(exitCodes) != 1 {
		fmt.Fprintf(w, "one exit code value expected, found %d in %v", len(exitCodes), exitCodes)

		return
	}

	exitCode, err := strconv.Atoi(exitCodes[0])
	if err != nil {
		fmt.Fprintf(w, "invalid exit code value: %s", exitCodes)

		return
	}

	os.Exit(exitCode)
}
