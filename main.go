package main

import (
	"encoding/json"
	stdErrors "errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"mcAfkGo/auth"
	"mcAfkGo/bot"
	"mcAfkGo/bot/basic"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

var (
	address   = getEnv("MC_ADDRESS", "127.0.0.1:25565")
	clientID  = getEnv("MS_CLIENT_ID", "")
	tokenFile = getEnv("MS_TOKEN_FILE", "token.mctoken")
)

var (
	client *bot.Client
	player *basic.Player
)

func startBot(startGameLoop bool) error {
	accessToken, playerID, name, err := auth.GetMinecraftToken(clientID, tokenFile)
	if err != nil {
		return err
	}

	client = bot.NewClient()
	client.Auth = bot.Auth{
		Name: name,
		UUID: playerID,
		AsTk: accessToken,
	}

	player = basic.NewPlayer(client, basic.DefaultSettings, basic.EventsListener{
		Death: onDeath,
	})

	if isPlayerOnline(address, name) {
		log.Println("Player is already online, bot will wait till player leaves.")

		for {
			time.Sleep(time.Minute)
			if !isPlayerOnline(address, name) {
				log.Println("Player is now offline, bot will join.")

				break
			}
		}
	}

	err = client.JoinServer(address)
	if err != nil {
		return err
	}

	log.Println("Joined server")

	if startGameLoop {
		for {
			err := client.HandleGame()
			if err == nil {
				panic("HandleGame never return nil")
			}

			if stdErrors.Is(err, io.EOF) {
				log.Println("Bot disconnected (EOF or disconnect). This usually means the account was logged in elsewhere or kicked.")
				for {
					time.Sleep(time.Minute)
					if !isPlayerOnline(address, client.Auth.Name) {
						log.Println("Player is offline, attempting to reconnect...")
						err := startBot(false)
						if err != nil {
							log.Printf("Reconnect failed: %v", err)
						} else {
							log.Println("Reconnected successfully!")
							break
						}
					}
				}
				continue
			}

			if err2 := new(bot.PacketHandlerError); stdErrors.As(err, err2) {
				log.Print(err2)
			} else {
				log.Fatalf("Unexpected error: %v", err)
			}
		}
	}

	return nil
}

func startAPI() {
	go func() {
		http.HandleFunc("/online-players", func(w http.ResponseWriter, r *http.Request) {
			players, err := GetOnlinePlayers(address)
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
			err = json.NewEncoder(w).Encode(players)
			if err != nil {
				log.Println("Failed to encode online players:", err)
			}
		})

		log.Println("API server listening on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
}

func main() {
	if clientID == "" {
		log.Fatal("MS_CLIENT_ID environment variable must be set. Get one from Azure AD app registration.")
	}

	startAPI()

	log.Println("Starting Microsoft authentication and bot...")
	err := startBot(true)
	if err != nil {
		log.Fatalf("Startup failed: %v", err)
	}

	log.Println("Login success")
}

func onDeath() error {
	log.Println("Died and Respawned")

	go func() {
		time.Sleep(time.Second * 5)
		err := player.Respawn()
		if err != nil {
			log.Print(err)
		}
	}()

	return nil
}

func isPlayerOnline(address, playerName string) bool {
	players, err := GetOnlinePlayers(address)
	if err != nil {
		return false
	}

	for _, n := range players {
		if n == playerName {
			return true
		}
	}

	return false
}
