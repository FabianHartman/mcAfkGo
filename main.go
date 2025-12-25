package main

import (
	"errors"
	"log"
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

func main() {
	// Check if client ID is provided
	if clientID == "" {
		log.Fatal("MS_CLIENT_ID environment variable must be set. Get one from Azure AD app registration.")
	}

	// Perform OAuth flow to get Minecraft token and profile
	log.Println("Starting Microsoft authentication...")
	accessToken, playerID, name, err := auth.GetMinecraftToken(
		clientID,
		tokenFile,
	)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	log.Printf("Authenticated as %s (UUID: %s)", name, playerID)

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

	err = client.JoinServer(address)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Login success")

	for {
		if err = client.HandleGame(); err == nil {
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
}

type DisconnectErr struct {
	Reason chat.Message
}

func (d DisconnectErr) Error() string {
	return "disconnect: " + d.Reason.ClearString()
}

func onDisconnect(reason chat.Message) error {
	// return an error value so that we can stop main loop
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
