package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/abibby/backup/backend"
	"github.com/gorilla/mux"
)

type Server struct {
	Port int
}

func greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World! %s", time.Now())
}
func files() func(w http.ResponseWriter, r *http.Request) {
	b, _ := backend.Load("file://./backup-folder")

	return func(w http.ResponseWriter, r *http.Request) {
		if files, err := b.List(r.URL.Path); err == nil {

			f := []string{}

			for _, file := range files {
				name := file.Name()
				if file.IsDir() {
					name += "/"
				}
				f = append(f, name)
			}

			sort.Strings(f)

			_ = json.NewEncoder(w).Encode(f)
			return
		}
		if file, err := b.Read(r.URL.Path); err == nil {
			if version := r.URL.Query().Get("version"); version != "" {
				t, err := time.Parse(time.RFC3339, version)
				if err != nil {
					w.WriteHeader(400)
					fmt.Fprintf(w, "%v", err)
					return
				}
				fileReader, err := file.Data(t)
				if err != nil {
					w.WriteHeader(500)
					fmt.Fprintf(w, "%v", err)
					return
				}
				_, err = io.Copy(w, fileReader)
				if err != nil {
					w.WriteHeader(500)
					fmt.Fprintf(w, "%v", err)
					return
				}
			} else {
				out := map[string]interface{}{
					"name":     file.Name(),
					"versions": file.Versions(),
				}
				_ = json.NewEncoder(w).Encode(out)
			}
			return
		}

		w.WriteHeader(404)
		// fmt.Fprintf(w, "files %s", r.URL.Path)
	}
}

func Start(s *Server) error {
	http.HandleFunc("/", greet)

	fmt.Printf("listening at http://localhost:%d\n", s.Port)
	r := mux.NewRouter()
	r.PathPrefix("/files").Handler(http.StripPrefix("/files", http.HandlerFunc(files())))
	http.ListenAndServe(fmt.Sprintf(":%d", s.Port), r)
	return nil
}
