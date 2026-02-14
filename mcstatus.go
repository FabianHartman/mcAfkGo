package main

import (
	"encoding/json"
	"net"
	"strconv"
	"time"
)

func packVarInt(val int) []byte {
	var result []byte
	for {
		temp := byte(val & 0x7F)
		val >>= 7
		if val != 0 {
			temp |= 0x80
		}
		result = append(result, temp)
		if val == 0 {
			break
		}
	}
	return result
}

func readVarIntConn(conn net.Conn) (int, error) {
	var num int
	var shift uint
	for {
		b := make([]byte, 1)
		_, err := conn.Read(b)
		if err != nil {
			return 0, err
		}
		num |= int(b[0]&0x7F) << shift
		if b[0]&0x80 == 0 {
			break
		}
		shift += 7
	}
	return num, nil
}

func GetOnlinePlayers(address string) ([]string, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, err
	}

	defer func() { _ = conn.Close() }()

	protocolVersion := 758
	hostBytes := []byte(host)
	handshake := append([]byte{}, packVarInt(0)...)
	handshake = append(handshake, packVarInt(protocolVersion)...)
	handshake = append(handshake, packVarInt(len(hostBytes))...)
	handshake = append(handshake, hostBytes...)
	handshake = append(handshake, byte(port>>8), byte(port&0xFF))
	handshake = append(handshake, packVarInt(1)...)

	handshakePacket := append(packVarInt(len(handshake)), handshake...)
	_, err = conn.Write(handshakePacket)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write([]byte{1, 0})
	if err != nil {
		return nil, err
	}

	_, err = readVarIntConn(conn)
	if err != nil {
		return nil, err
	}
	_, err = readVarIntConn(conn)
	if err != nil {
		return nil, err
	}

	strLen, err := readVarIntConn(conn)
	if err != nil {
		return nil, err
	}

	data := make([]byte, strLen)
	read := 0

	for read < strLen {
		n, err := conn.Read(data[read:])
		if err != nil {
			return nil, err
		}

		read += n
	}

	var status struct {
		Players struct {
			Sample []struct {
				Name string `json:"name"`
			} `json:"sample"`
		} `json:"players"`
	}

	err = json.Unmarshal(data, &status)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, p := range status.Players.Sample {
		names = append(names, p.Name)
	}

	return names, nil
}
