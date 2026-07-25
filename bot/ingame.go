package bot

import (
	"errors"
	"fmt"

	"mcAfkGo/data/packetid"
	pk "mcAfkGo/net/packet"
)

func (c *Client) HandleGame() error {
	for {
		var packet pk.Packet

		err := c.Conn.ReadPacket(&packet)
		if err != nil {
			return err
		}

		if packet.ID == int32(packetid.BundleDelimiter) {
			err := c.handleBundlePackets()
			if err != nil {
				return err
			}
		} else {
			err := c.handlePacket(packet)
			if err != nil {
				return err
			}

			c.Conn.pool.Put(packet.Data)
		}
	}
}

type PacketHandlerError struct {
	ID  packetid.ClientboundPacketID
	Err error
}

func (d PacketHandlerError) Error() string {
	return fmt.Sprintf("handle packet %v error: %v", d.ID, d.Err)
}

func (c *Client) handleBundlePackets() (err error) {
	var packets []pk.Packet
	for i := 0; i < 4096; i++ {
		var packet pk.Packet
		err := c.Conn.ReadPacket(&packet)
		if err != nil {
			return err
		}

		if packet.ID == int32(packetid.BundleDelimiter) {
			goto handlePackets
		}

		packets = append(packets, packet)
	}

	return errors.New("packet number of a bundle out of limit")

handlePackets:
	for i := range packets {
		err := c.handlePacket(packets[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) handlePacket(p pk.Packet) (err error) {
	packetID := packetid.ClientboundPacketID(p.ID)
	for _, handler := range c.Events.generic {
		err = handler.F(p)
		if err != nil {
			return PacketHandlerError{ID: packetID, Err: err}
		}
	}

	for _, handler := range c.Events.handlers[packetID] {
		err = handler.F(p)
		if err != nil {
			return PacketHandlerError{ID: packetID, Err: err}
		}
	}

	return
}
