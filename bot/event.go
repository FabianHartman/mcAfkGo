package bot

import (
	"sort"
	"strconv"

	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

type Events struct {
	generic  []PacketHandler   `desc:"for every packet"`
	handlers [][]PacketHandler `desc:"for specific packet id only"`
}

func (e *Events) AddListener(listeners ...PacketHandler) {
	for _, listener := range listeners {
		if listener.ID < 0 || int(listener.ID) >= len(e.handlers) {
			panic("Invalid packet ID (" + strconv.Itoa(int(listener.ID)) + ")")
		}

		s := e.handlers[listener.ID]
		if s == nil {
			e.handlers[listener.ID] = []PacketHandler{listener}
		} else {
			e.handlers[listener.ID] = append(s, listener)
			sortPacketHandlers(e.handlers[listener.ID])
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
