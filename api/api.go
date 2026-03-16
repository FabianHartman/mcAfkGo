package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// StartAPI starts the HTTP API server on :8080.
func StartAPI(address string, getPlayers func(string) ([]string, error)) {
	go func() {
		http.HandleFunc("/online-players", onlinePlayersHandler(address, getPlayers))
		http.HandleFunc("/online-players/v2", onlinePlayersV2Handler(address, getPlayers))

		log.Println("API server listening on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
}

// onlinePlayersHandler returns an http.HandlerFunc that writes the players slice as JSON.
func onlinePlayersHandler(address string, getPlayers func(string) ([]string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		players, err := getPlayers(address)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, responseWriteErr := w.Write([]byte(`{"error": "Failed to get online players"}`))
			if responseWriteErr != nil {
				log.Println("Failed to write error response:", responseWriteErr)
			}

			log.Println("Failed to get online players:", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(players); err != nil {
			log.Println("Failed to encode online players:", err)
		}
	}
}

// onlinePlayersV2Handler returns an http.HandlerFunc that writes the players wrapped in an object {"players": [...] }.
func onlinePlayersV2Handler(address string, getPlayers func(string) ([]string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		players, err := getPlayers(address)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, responseWriteErr := w.Write([]byte(`{"error": "Failed to get online players"}`))
			if responseWriteErr != nil {
				log.Println("Failed to write error response:", responseWriteErr)
			}

			log.Println("Failed to get online players:", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := struct {
			Players []string `json:"players"`
		}{Players: players}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Println("Failed to encode online players v2:", err)
		}
	}
}
