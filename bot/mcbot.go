package bot

import (
	"context"
	"errors"
	"net"
	"strconv"

	"mcAfkGo/auth/user"
	"mcAfkGo/chat"
	mcnet "mcAfkGo/net"
	pk "mcAfkGo/net/packet"
	"mcAfkGo/net/queue"
)

const ProtocolVersion = 767

type JoinOptions struct {
	MCDialer mcnet.MCDialer
	Context  context.Context

	NoPublicKey bool

	KeyPair *user.KeyPairResp

	QueueRead  queue.Queue[pk.Packet]
	QueueWrite queue.Queue[pk.Packet]
}

func (c *Client) JoinServer(addr string) (err error) {
	return c.JoinServerWithOptions(addr, JoinOptions{})
}

func (c *Client) JoinServerWithOptions(addr string, options JoinOptions) (err error) {
	if options.MCDialer == nil {
		options.MCDialer = &mcnet.DefaultDialer
	}
	if options.Context == nil {
		options.Context = context.Background()
	}
	if options.QueueRead == nil {
		options.QueueRead = queue.NewLinkedQueue[pk.Packet]()
	}
	if options.QueueWrite == nil {
		options.QueueWrite = queue.NewLinkedQueue[pk.Packet]()
	}
	return c.join(addr, options)
}

func (c *Client) join(addr string, options JoinOptions) error {
	const Handshake = 0x00

	host, portStr, err := net.SplitHostPort(addr)
	var port uint64
	if err != nil {
		var addrErr *net.AddrError
		const missingPort = "missing port in address"
		if errors.As(err, &addrErr) && addrErr.Err == missingPort {
			host = addr
			port = 25565
		} else {
			return LoginErr{"split address", err}
		}
	} else {
		port, err = strconv.ParseUint(portStr, 0, 16)
		if err != nil {
			return LoginErr{"parse port", err}
		}
	}

	conn, err := options.MCDialer.DialMCContext(options.Context, addr)
	if err != nil {
		return LoginErr{"connect server", err}
	}

	err = conn.WritePacket(pk.Marshal(
		Handshake,
		pk.VarInt(ProtocolVersion), // Protocol version
		pk.String(host),            // Host
		pk.UnsignedShort(port),     // Port
		pk.VarInt(2),
	))
	if err != nil {
		return LoginErr{"handshake", err}
	}

	if err := c.joinLogin(conn); err != nil {
		return err
	}

	if err := c.joinConfiguration(conn); err != nil {
		return err
	}

	c.Conn = warpConn(conn, options.QueueRead, options.QueueWrite)

	return nil
}

type DisconnectErr chat.Message

func (d DisconnectErr) Error() string {
	return "disconnect because: " + chat.Message(d).String()
}
