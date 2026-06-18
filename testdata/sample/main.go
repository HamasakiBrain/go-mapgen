// Command sample is a tiny service used to exercise mapgen.
package main

import (
	"log"
	"net/http"

	"example.com/sample/api"
	"example.com/sample/store"
)

func main() {
	srv := buildServer()
	mux := http.NewServeMux()
	srv.Register(mux)
	log.Fatal(http.ListenAndServe(":9090", mux))
}

// buildServer wires the store into the API server.
func buildServer() *api.Server {
	ms := store.NewMemStore()
	seed(ms)
	return api.NewServer(ms)
}

// seed inserts initial data via the interface.
func seed(s api.Store) {
	_ = s.Save("hello", "world")
}
