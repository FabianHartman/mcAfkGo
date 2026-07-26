package frontend

import (
	_ "embed"
	"log"
	"net/http"
)

//go:embed dist/index.html
var indexHTML []byte

func IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		_, err := w.Write(indexHTML)
		if err != nil {
			log.Println("Failed to write index response:", err)
		}
	}
}
