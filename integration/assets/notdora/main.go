package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", HelloServer)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

func HelloServer(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hi, I'm not Dora!")
}
