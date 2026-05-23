package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func StartAPI(address string, getPlayers func(string) ([]string, error), getLastSeen func() map[string]time.Time) {
	go func() {
		http.HandleFunc("/online-players", onlinePlayersHandler(address, getPlayers))
		http.HandleFunc("/online-players/v2", onlinePlayersV2Handler(address, getPlayers))
		http.HandleFunc("/players/last-seen", lastSeenHandler(getLastSeen))

		log.Println("API server listening on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
}

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

func lastSeenHandler(getLastSeen func() map[string]time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot := getLastSeen()

		out := make(map[string]string, len(snapshot))
		for k, v := range snapshot {
			out[k] = v.Format(time.RFC3339)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(out); err != nil {
			log.Println("Failed to encode last-seen:", err)
		}
	}
}
