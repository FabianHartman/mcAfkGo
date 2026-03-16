package main

import (
	stdErrors "errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"mcAfkGo/api"
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

	playerIsOnline, err := isPlayerOnline(address, name)
	if err != nil {
		fmt.Printf("failed to check if player is online: %v\n", err)
		fmt.Println("Bot will try again in a minute")
	}

	if playerIsOnline {
		log.Println("Player is already online, bot will wait till player leaves.")
	}

	if playerIsOnline || err != nil {
		for {
			time.Sleep(time.Minute)

			playerIsOnline, err = isPlayerOnline(address, name)
			if err != nil {
				fmt.Printf("failed to check if player is online: %v\n", err)
				fmt.Println("Bot will try again in a minute")
			} else if !playerIsOnline {
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

					playerIsOnline, err := isPlayerOnline(address, client.Auth.Name)
					if err != nil {
						fmt.Printf("failed to check if player is online: %v\n", err)
						fmt.Println("Bot will try again in a minute")
					} else if !playerIsOnline {
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

func main() {
	if clientID == "" {
		log.Fatal("MS_CLIENT_ID environment variable must be set. Get one from Azure AD app registration.")
	}

	api.StartAPI(address, GetOnlinePlayers)

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

func isPlayerOnline(address, playerName string) (bool, error) {
	players, err := GetOnlinePlayers(address)
	if err != nil {
		return false, fmt.Errorf("failed to get online players: %w", err)
	}

	for _, n := range players {
		if n == playerName {
			return true, nil
		}
	}

	return false, nil
}
