package basic

import (
	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

func (p *Player) handleCookieRequestPacket(packet pk.Packet) error {
	var key pk.Identifier
	err := packet.Scan(&key)
	if err != nil {
		return Error{err}
	}

	cookieContent := p.c.Cookies[string(key)]
	err = p.c.Conn.WritePacket(pk.Marshal(
		packetid.ServerboundCookieResponse,
		key, pk.OptionEncoder[pk.ByteArray]{
			Has: cookieContent != nil,
			Val: cookieContent,
		},
	))
	if err != nil {
		return Error{err}
	}

	return nil
}

func (p *Player) handleStoreCookiePacket(packet pk.Packet) error {
	var key pk.Identifier
	var payload pk.ByteArray
	err := packet.Scan(&key, &payload)
	if err != nil {
		return Error{err}
	}

	p.c.Cookies[string(key)] = payload

	return nil
}
