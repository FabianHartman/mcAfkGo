package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"mcAfkGo/auth"
	"mcAfkGo/bot"
	"mcAfkGo/bot/basic"
	"mcAfkGo/chat"
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
		Disconnect: onDisconnect,
		Death:      onDeath,
	})

	if isPlayerOnline(address, name) {
		err = onDisconnect(chat.Message{
			Text: "Player is already online",
		})
		if err != nil {
			return err
		}
	}

	err = client.JoinServer(address)
	if err != nil {
		return err
	}

	if startGameLoop {
		go func() {
			for {
				err := client.HandleGame()
				if err == nil {
					panic("HandleGame never return nil")
				}

				if err2 := new(bot.PacketHandlerError); errors.As(err, err2) {
					if err := new(DisconnectErr); errors.As(err2, err) {
						log.Print("Disconnect, reason: ", err.Reason)

						return
					} else {
						log.Print(err2)
					}
				} else {
					log.Fatal(err)
				}
			}
		}()
	}

	return nil
}

func startAPI() {
	go func() {
		http.HandleFunc("/api/online-players", func(w http.ResponseWriter, r *http.Request) {
			players, err := GetOnlinePlayers(address)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, err = w.Write([]byte(`{"error": "Failed to get online players"}`))
				if err != nil {
					log.Println("Failed to write error response:", err)
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

	select {} // block forever, game loop is in goroutine
}

type DisconnectErr struct {
	Reason chat.Message
}

func (d DisconnectErr) Error() string {
	return "disconnect: " + d.Reason.ClearString()
}

var reconnecting = false

func onDisconnect(reason chat.Message) error {
	go func() {
		if reconnecting {
			return
		}

		reconnecting = true

		defer func() { reconnecting = false }()

		for {
			time.Sleep(time.Minute)

			isOnline := isPlayerOnline(address, client.Auth.Name)
			if !isOnline {
				log.Println("Player is offline, attempting to reconnect...")
				err := startBot(true)
				if err != nil {
					log.Printf("Reconnect failed: %v", err)
				} else {
					log.Println("Reconnected successfully!")
					return
				}
			}
		}
	}()

	return DisconnectErr{Reason: reason}
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
