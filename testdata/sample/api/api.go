// Package api exposes an HTTP layer over a Store.
package api

import (
	"fmt"
	"net/http"
)

// Store is the persistence contract the API depends on.
type Store interface {
	Get(id string) (string, error)
	Save(id, val string) error
}

// Server wires HTTP handlers to a Store.
type Server struct {
	store Store
}

// NewServer builds a Server backed by the given Store.
func NewServer(s Store) *Server {
	return &Server{store: s}
}

// Register attaches routes to the mux. Route detection should find these.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/get", s.HandleGet)
	mux.HandleFunc("/save", s.HandleSave)
}

// HandleGet reads a value. Dispatch to store.Get goes through the interface,
// so only a type-checked call graph resolves it to *store.MemStore.Get.
func (s *Server) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	val, err := s.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	fmt.Fprint(w, val)
}

// HandleSave writes a value.
func (s *Server) HandleSave(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	val := r.URL.Query().Get("val")
	if err := s.store.Save(id, val); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "ok")
}

// classify is an intentionally convoluted function to exercise the
// complexity and smell detectors.
func classify(a, b, c, d, e, f int) string {
	out := ""
	for i := 0; i < a; i++ {
		if b > 0 && c > 0 {
			for j := 0; j < b; j++ {
				switch {
				case j%2 == 0 && d > 0:
					out += "x"
				case j%3 == 0 || e > 0:
					out += "y"
				default:
					out += "z"
				}
			}
		} else if f > 0 {
			out += "f"
		}
	}
	return out
}

var _ = classify
