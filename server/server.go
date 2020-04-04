package server

import (
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	Port int
}

func greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World! %s", time.Now())
}

func Start(s *Server) error {
	http.HandleFunc("/", greet)

	fmt.Printf("listening at http://localhost:%d\n", s.Port)

	http.ListenAndServe(fmt.Sprintf(":%d", s.Port), nil)
	return nil
}
