package main

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"log"
	"os"
	"time"

	"mcAfkGo/bot"
	"mcAfkGo/bot/basic"
	"mcAfkGo/chat"
	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

var (
	address     = getEnv("MC_ADDRESS", "127.0.0.1:25565")
	name        = getEnv("MC_NAME", "AFKBot")
	playerID    = getEnv("MC_UUID", "")
	accessToken = getEnv("MC_TOKEN", "")
)

var (
	client *bot.Client
	player *basic.Player
)

func main() {
	client = bot.NewClient()
	client.Auth = bot.Auth{
		Name: name,
		UUID: playerID,
		AsTk: accessToken,
	}
	player = basic.NewPlayer(client, basic.DefaultSettings, basic.EventsListener{
		GameStart:  onGameStart,
		Disconnect: onDisconnect,
		Death:      onDeath,
	})

	err := client.JoinServer(address)
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

func onGameStart() error {
	log.Println("Bot joined the server, sending greeting message...")

	message := "Hello! I'm an AFK bot."
	err := sendChatMessage(message)
	if err != nil {
		log.Printf("Failed to send chat message: %v", err)

		return err
	}

	return nil
}

func sendChatMessage(msg string) error {
	if len(msg) > 256 {
		return errors.New("message length greater than 256")
	}

	var salt int64
	if err := binary.Read(rand.Reader, binary.BigEndian, &salt); err != nil {
		return err
	}

	err := client.Conn.WritePacket(pk.Marshal(
		packetid.ServerboundChat,
		pk.String(msg),
		pk.Long(time.Now().UnixMilli()),
		pk.Long(salt),
		pk.Boolean(false),     // has signature
		pk.VarInt(0),          // offset
		pk.NewFixedBitSet(20), // acknowledged bit set (20 bits)
	))
	return err
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
