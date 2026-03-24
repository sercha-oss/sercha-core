package main

import (
	"fmt"
	"log"
	"net/http"
)

// Server represents the main application server
type Server struct {
	port int
	name string
}

// NewServer creates a new server instance
func NewServer(port int, name string) *Server {
	return &Server{
		port: port,
		name: name,
	}
}

// Start initializes and starts the HTTP server
func (s *Server) Start() error {
	http.HandleFunc("/", s.handleRoot)
	http.HandleFunc("/health", s.handleHealth)
	http.HandleFunc("/api/search", s.handleSearch)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting %s server on %s", s.name, addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to %s!", s.name)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","service":"%s"}`, s.name)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"query":"%s","results":[]}`, query)
}

func main() {
	server := NewServer(8080, "Project Alpha")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
