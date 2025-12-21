package basic

import (
	"bytes"
	"io"

	pk "mcAfkGo/net/packet"
)

func (p *Player) handleUpdateTags(packet pk.Packet) error {
	r := bytes.NewReader(packet.Data)

	var length pk.VarInt
	_, err := length.ReadFrom(r)
	if err != nil {
		return Error{err}
	}

	var registryID pk.Identifier
	for i := 0; i < int(length); i++ {
		_, err = registryID.ReadFrom(r)
		if err != nil {
			return Error{err}
		}

		registry := p.c.Registries.Registry(string(registryID))
		if registry == nil {
			// Skip unknown registries - we don't need them
			_, err = idleTagsDecoder{}.ReadFrom(r)
			if err != nil {
				return Error{err}
			}
			continue
		}

		_, err = registry.ReadTagsFrom(r)
		if err != nil {
			return Error{err}
		}
	}
	return nil
}

type idleTagsDecoder struct{}

func (idleTagsDecoder) ReadFrom(r io.Reader) (int64, error) {
	var count pk.VarInt
	var tag pk.Identifier
	var length pk.VarInt
	n, err := count.ReadFrom(r)
	if err != nil {
		return n, err
	}
	for i := 0; i < int(count); i++ {
		var n1, n2, n3 int64
		n1, err = tag.ReadFrom(r)
		if err != nil {
			return n + n1, err
		}
		n2, err = length.ReadFrom(r)
		if err != nil {
			return n + n1 + n2, err
		}
		n += n1 + n2

		var id pk.VarInt
		for i := 0; i < int(length); i++ {
			n3, err = id.ReadFrom(r)
			if err != nil {
				return n + n3, err
			}
			n += n3
		}
	}
	return n, nil
}
