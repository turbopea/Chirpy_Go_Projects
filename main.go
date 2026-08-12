package main

import (
	"log"
	"net/http"
)

func main() {
	servemux := http.NewServeMux()
	s := servemux.Handle("/", http.FileServer(http.Dir(".")))

	s := &http.Server{
		Addr:    ":8080",
		Handler: s,
	}

	log.Fatal(s.ListenAndServe())
}
