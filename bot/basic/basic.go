package basic

import (
	"mcAfkGo/bot"
	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

type Player struct {
	c        *bot.Client
	Settings Settings

	PlayerInfo
	WorldInfo
}

func NewPlayer(c *bot.Client, settings Settings, events EventsListener) *Player {
	p := &Player{c: c, Settings: settings}
	c.Events.AddListener(
		bot.PacketHandler{Priority: 0, ID: packetid.ClientboundLogin, F: p.handleLoginPacket},
		bot.PacketHandler{Priority: 0, ID: packetid.ClientboundKeepAlive, F: p.handleKeepAlivePacket},
		bot.PacketHandler{Priority: 0, ID: packetid.ClientboundRespawn, F: p.handleRespawnPacket},
		bot.PacketHandler{Priority: 0, ID: packetid.ClientboundPing, F: p.handlePingPacket},
		bot.PacketHandler{Priority: 0, ID: packetid.ClientboundCookieRequest, F: p.handleCookieRequestPacket},
		bot.PacketHandler{Priority: 0, ID: packetid.ClientboundStoreCookie, F: p.handleStoreCookiePacket},
		bot.PacketHandler{Priority: 0, ID: packetid.ClientboundUpdateTags, F: p.handleUpdateTags},
	)
	events.attach(p)
	return p
}

func (p *Player) Respawn() error {
	const PerformRespawn = 0

	err := p.c.Conn.WritePacket(pk.Marshal(
		packetid.ServerboundClientCommand,
		pk.VarInt(PerformRespawn),
	))
	if err != nil {
		return Error{err}
	}

	return nil
}

type Error struct {
	Err error
}

func (e Error) Error() string {
	return "bot/basic: " + e.Err.Error()
}
