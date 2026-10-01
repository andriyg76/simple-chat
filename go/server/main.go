package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	addr := ":8080"
	log.Printf("simple-chat go server listening on %s (stub; see ../protocol.md)", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
