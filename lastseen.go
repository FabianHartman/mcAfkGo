package main

import (
	"log"
	"time"
)

var LastSeen = make(map[string]time.Time)

func StartLastSeenPoller(address string) {
	go func() {
		updateLastSeen(address)

		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			updateLastSeen(address)
		}
	}()
}

func updateLastSeen(address string) {
	players, err := GetOnlinePlayers(address)
	if err != nil {
		log.Printf("lastseen poll: failed to get online players: %v", err)
		return
	}

	now := time.Now()

	for _, p := range players {
		LastSeen[p] = now
	}
}

func GetLastSeen() map[string]time.Time {
	return LastSeen
}
