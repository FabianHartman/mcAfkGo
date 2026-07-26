// Package net pack network connection for Minecraft.
package net

import (
	"context"
	"crypto/cipher"
	"errors"
	"io"
	"net"
	"strconv"
	"time"

	pk "mcAfkGo/net/packet"
)

const DefaultPort = 25565

type Listener struct{ net.Listener }

type Conn struct {
	Socket net.Conn
	io.Reader
	io.Writer

	threshold int
}

var DefaultDialer = Dialer{}

type MCDialer interface {
	DialMCContext(ctx context.Context, addr string) (*Conn, error)
}

type Dialer net.Dialer

func (d *Dialer) resolver() *net.Resolver {
	if d != nil && d.Resolver != nil {
		return d.Resolver
	}

	return net.DefaultResolver
}

func (d *Dialer) DialMCContext(ctx context.Context, addr string) (*Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		var addrErr *net.AddrError
		const missingPort = "missing port in address"
		if errors.As(err, &addrErr) && addrErr.Err == missingPort {
			host, port, err = addr, "", nil
		} else {
			return nil, err
		}
	}

	var addresses []string
	if port == "" {
		_, srvRecords, err := d.resolver().LookupSRV(ctx, "minecraft", "tcp", host)
		if err == nil {
			for _, record := range srvRecords {
				addr := net.JoinHostPort(record.Target, strconv.Itoa(int(record.Port)))
				addresses = append(addresses, addr)
			}
		}

		addr = net.JoinHostPort(addr, strconv.Itoa(DefaultPort))
	}
	addresses = append(addresses, addr)

	var firstErr error
	for i, addr := range addresses {
		select {
		case <-ctx.Done():
			return nil, context.Canceled
		default:
		}

		dialCtx := ctx
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			partialDeadline, err := partialDeadline(time.Now(), deadline, len(addresses)-i)
			if err != nil {
				if firstErr == nil {
					firstErr = context.DeadlineExceeded
				}

				break
			}

			if partialDeadline.Before(deadline) {
				var cancel context.CancelFunc

				dialCtx, cancel = context.WithDeadline(ctx, partialDeadline)

				defer cancel()
			}
		}

		conn, err := (*net.Dialer)(d).DialContext(dialCtx, "tcp", addr)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		return WrapConn(conn), nil
	}

	return nil, firstErr
}

func (d *Dialer) deadline(ctx context.Context, now time.Time) (earliest time.Time) {
	if d.Timeout != 0 {
		earliest = now.Add(d.Timeout)
	}

	deadline, ok := ctx.Deadline()
	if ok {
		earliest = minNonzeroTime(earliest, deadline)
	}

	return minNonzeroTime(earliest, d.Deadline)
}

func minNonzeroTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}

	if b.IsZero() || a.Before(b) {
		return a
	}

	return b
}

func partialDeadline(now, deadline time.Time, addrsRemaining int) (time.Time, error) {
	const saneMinimum = 2 * time.Second

	if deadline.IsZero() {
		return deadline, nil
	}

	timeRemaining := deadline.Sub(now)
	if timeRemaining <= 0 {
		return time.Time{}, context.DeadlineExceeded
	}

	timeout := timeRemaining / time.Duration(addrsRemaining)
	if timeout < saneMinimum {
		if timeRemaining < saneMinimum {
			timeout = timeRemaining
		} else {
			timeout = saneMinimum
		}
	}

	return now.Add(timeout), nil
}

func WrapConn(conn net.Conn) *Conn {
	return &Conn{
		Socket:    conn,
		Reader:    conn,
		Writer:    conn,
		threshold: -1,
	}
}

func (c *Conn) Close() error { return c.Socket.Close() }

func (c *Conn) ReadPacket(p *pk.Packet) error {
	return p.UnPack(c.Reader, c.threshold)
}

func (c *Conn) WritePacket(p pk.Packet) error {
	return p.Pack(c.Writer, c.threshold)
}

func (c *Conn) SetCipher(ecoStream, decoStream cipher.Stream) {
	c.Reader = cipher.StreamReader{
		S: decoStream,
		R: c.Socket,
	}

	c.Writer = cipher.StreamWriter{
		S: ecoStream,
		W: c.Socket,
	}
}

func (c *Conn) SetThreshold(t int) {
	c.threshold = t
}
