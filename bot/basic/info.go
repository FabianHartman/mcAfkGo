package basic

import (
	"unsafe"

	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

type WorldInfo struct {
	DimensionType       int32
	DimensionNames      []string `desc:"Identifiers for all worlds on the server"`
	DimensionName       string   `desc:"Name of the world being spawned into."`
	HashedSeed          int64    `desc:"First 8 bytes of the SHA-256 hash of the world's seed. Used client side for biome noise"`
	MaxPlayers          int32    `desc:"Was once used by the client to draw the player list, but now is ignored."`
	ViewDistance        int32    `desc:"Render distance (2-32)."`
	SimulationDistance  int32    `desc:"The distance that the client will process specific things, such as entities."`
	ReducedDebugInfo    bool     `desc:"If true, a vanilla client shows reduced information on the debug screen. For servers in development, this should almost always be false."`
	EnableRespawnScreen bool     `desc:"Set to false when the doImmediateRespawn GameRule is true."`
	IsDebug             bool     `desc:"True if the world is a debug mode world; debug mode worlds cannot be modified and have predefined blocks."`
	IsFlat              bool     `desc:"True if the world is a superflat world; flat worlds have different void fog and a horizon at y=0 instead of y=63."`
	DoLimitCrafting     bool     `desc:"Whether players can only craft recipes they have already unlocked. Currently unused by the client."`
}

type PlayerInfo struct {
	EID          int32 `desc:"The player's Entity ID (EID)."`
	Hardcore     bool  `desc:"Is hardcore"`
	Gamemode     byte  `desc:"GameMode. 0: Survival, 1: Creative, 2: Adventure, 3: Spectator."`
	PrevGamemode int8  `desc:"Previous GameMode"`
}

func (p *Player) handleLoginPacket(packet pk.Packet) error {
	err := packet.Scan(
		(*pk.Int)(&p.EID),
		(*pk.Boolean)(&p.Hardcore),
		pk.Array((*[]pk.Identifier)(unsafe.Pointer(&p.DimensionNames))),
		(*pk.VarInt)(&p.MaxPlayers),
		(*pk.VarInt)(&p.ViewDistance),
		(*pk.VarInt)(&p.SimulationDistance),
		(*pk.Boolean)(&p.ReducedDebugInfo),
		(*pk.Boolean)(&p.EnableRespawnScreen),
		(*pk.Boolean)(&p.DoLimitCrafting),
		(*pk.VarInt)(&p.WorldInfo.DimensionType),
		(*pk.Identifier)(&p.DimensionName),
		(*pk.Long)(&p.HashedSeed),
		(*pk.UnsignedByte)(&p.Gamemode),
		(*pk.Byte)(&p.PrevGamemode),
		(*pk.Boolean)(&p.IsDebug),
		(*pk.Boolean)(&p.IsFlat),
	)
	if err != nil {
		return Error{err}
	}

	err = p.c.Conn.WritePacket(pk.Marshal(
		packetid.ServerboundCustomPayload,
		pk.Identifier("minecraft:brand"),
		pk.String(p.Settings.Brand),
	))
	if err != nil {
		return Error{err}
	}

	err = p.c.Conn.WritePacket(pk.Marshal(
		packetid.ServerboundClientInformation,
		pk.String(p.Settings.Locale),
		pk.Byte(p.Settings.ViewDistance),
		pk.VarInt(p.Settings.ChatMode),
		pk.Boolean(p.Settings.ChatColors),
		pk.UnsignedByte(p.Settings.DisplayedSkinParts),
		pk.VarInt(p.Settings.MainHand),
		pk.Boolean(p.Settings.EnableTextFiltering),
		pk.Boolean(p.Settings.AllowListing),
	))
	if err != nil {
		return Error{err}
	}

	p.resetKeepAliveDeadline()

	return nil
}

func (p *Player) handleRespawnPacket(packet pk.Packet) error {
	var copyMeta bool
	err := packet.Scan(
		(*pk.VarInt)(&p.DimensionType),
		(*pk.Identifier)(&p.DimensionName),
		(*pk.Long)(&p.HashedSeed),
		(*pk.UnsignedByte)(&p.Gamemode),
		(*pk.Byte)(&p.PrevGamemode),
		(*pk.Boolean)(&p.IsDebug),
		(*pk.Boolean)(&p.IsFlat),
		(*pk.Boolean)(&copyMeta),
	)
	if err != nil {
		return Error{err}
	}

	return nil
}
