package bot

import (
	"errors"
	"sync"

	"github.com/google/uuid"

	"mcAfkGo/data/packetid"
	"mcAfkGo/net"
	pk "mcAfkGo/net/packet"
	"mcAfkGo/net/queue"
	"mcAfkGo/registry"
)

type Client struct {
	Conn *Conn
	Auth Auth

	Name       string
	UUID       uuid.UUID
	Registries registry.Registries
	Cookies    map[string][]byte

	Events Events

	LoginPlugin map[string]CustomPayloadHandler

	ConfigHandler

	CustomReportDetails map[string]string
}

type CustomPayloadHandler func(data []byte) ([]byte, error)

func (c *Client) Close() error {
	return c.Conn.Close()
}

func NewClient() *Client {
	return &Client{
		Auth:                Auth{Name: "Steve"},
		Registries:          registry.NewNetworkCodec(),
		Events:              Events{handlers: make([][]PacketHandler, packetid.ClientboundPacketIDGuard)},
		LoginPlugin:         make(map[string]CustomPayloadHandler),
		ConfigHandler:       NewDefaultConfigHandler(),
		CustomReportDetails: make(map[string]string),
	}
}

type Conn struct {
	*net.Conn
	send, recv queue.Queue[pk.Packet]
	pool       sync.Pool
	rerr       error
}

func warpConn(c *net.Conn, qr, qw queue.Queue[pk.Packet]) *Conn {
	wc := Conn{
		Conn: c,
		send: qw,
		recv: qr,
		pool: sync.Pool{New: func() any { return []byte{} }},
		rerr: nil,
	}
	go func() {
		for {
			p := pk.Packet{Data: wc.pool.Get().([]byte)}
			if err := c.ReadPacket(&p); err != nil {
				wc.rerr = err
				break
			}
			if ok := wc.recv.Push(p); !ok {
				wc.rerr = errors.New("receive queue is full")
				break
			}
		}
		wc.recv.Close()
	}()
	go func() {
		for {
			p, ok := wc.send.Pull()
			if !ok {
				break
			}
			if err := c.WritePacket(p); err != nil {
				break
			}
		}
	}()

	return &wc
}

func (c *Conn) ReadPacket(p *pk.Packet) error {
	packet, ok := c.recv.Pull()
	if !ok {
		return c.rerr
	}
	*p = packet
	return nil
}

func (c *Conn) WritePacket(p pk.Packet) error {
	ok := c.send.Push(p)
	if !ok {
		return errors.New("queue is full")
	}
	return nil
}

func (c *Conn) Close() error {
	c.send.Close()
	return c.Conn.Close()
}
