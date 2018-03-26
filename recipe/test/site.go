package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/requesturi/", echo)
	fmt.Println("listening...")
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}

func hello(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(res, "go, world")
}

func echo(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(res, fmt.Sprintf("Request URI is [%s]\nQuery String is [%s]", req.RequestURI, req.URL.RawQuery))
}
