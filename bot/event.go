package bot

import (
	"sort"
	"strconv"

	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

type Events struct {
	generic  []PacketHandler   // for every packet
	handlers [][]PacketHandler // for specific packet id only
}

func (e *Events) AddListener(listeners ...PacketHandler) {
	for _, l := range listeners {
		// panic if l.ID is invalid
		if l.ID < 0 || int(l.ID) >= len(e.handlers) {
			panic("Invalid packet ID (" + strconv.Itoa(int(l.ID)) + ")")
		}
		if s := e.handlers[l.ID]; s == nil {
			e.handlers[l.ID] = []PacketHandler{l}
		} else {
			e.handlers[l.ID] = append(s, l)
			sortPacketHandlers(e.handlers[l.ID])
		}
	}
}

type PacketHandler struct {
	ID       packetid.ClientboundPacketID
	Priority int
	F        func(p pk.Packet) error
}

func sortPacketHandlers(slice []PacketHandler) {
	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].Priority > slice[j].Priority
	})
}
