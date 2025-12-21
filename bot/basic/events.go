package basic

import (
	"mcAfkGo/bot"
	"mcAfkGo/chat"
	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

type EventsListener struct {
	GameStart  func() error
	Disconnect func(reason chat.Message) error
	Death      func() error
}

func (e EventsListener) attach(p *Player) {
	if e.GameStart != nil {
		attachJoinGameHandler(p.c, e.GameStart)
	}
	if e.Disconnect != nil {
		attachDisconnect(p.c, e.Disconnect)
	}
}

func attachJoinGameHandler(c *bot.Client, handler func() error) {
	c.Events.AddListener(bot.PacketHandler{
		Priority: 64, ID: packetid.ClientboundLogin,
		F: func(_ pk.Packet) error {
			return handler()
		},
	})
}

func attachDisconnect(c *bot.Client, handler func(reason chat.Message) error) {
	c.Events.AddListener(bot.PacketHandler{
		Priority: 64, ID: packetid.ClientboundDisconnect,
		F: func(p pk.Packet) error {
			var reason chat.Message
			if err := p.Scan(&reason); err != nil {
				return Error{err}
			}
			return handler(chat.Message(reason))
		},
	})
}
