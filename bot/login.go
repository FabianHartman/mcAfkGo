package bot

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mcAfkGo/chat"
	"mcAfkGo/data/packetid"
	"mcAfkGo/net"
	"mcAfkGo/net/CFB8"
	pk "mcAfkGo/net/packet"
)

type LoginErr struct {
	Stage string
	Err   error
}

func (l LoginErr) Error() string {
	return "bot: login error: [" + l.Stage + "] " + l.Err.Error()
}

func (c *Client) joinLogin(conn *net.Conn) error {
	var err error
	if c.Auth.UUID != "" {
		c.UUID, err = uuid.Parse(c.Auth.UUID)
		if err != nil {
			return LoginErr{"login start", err}
		}
	}

	err = conn.WritePacket(pk.Marshal(
		packetid.ServerboundLoginHello,
		pk.String(c.Auth.Name),
		pk.UUID(c.UUID),
	))
	if err != nil {
		return LoginErr{"login start", err}
	}

	receiving := "encrypt start"

	for {
		var p pk.Packet
		err = conn.ReadPacket(&p)
		if err != nil {
			return LoginErr{receiving, err}
		}

		switch packetid.ClientboundPacketID(p.ID) {
		case packetid.ClientboundLoginLoginDisconnect:
			var reason chat.JsonMessage
			err = p.Scan(&reason)
			if err != nil {
				return LoginErr{"disconnect", err}
			}

			return LoginErr{"disconnect", DisconnectErr(reason)}

		case packetid.ClientboundLoginHello:
			err := handleEncryptionRequest(conn, c, p)
			if err != nil {
				return LoginErr{"encryption", err}
			}

			receiving = "set compression"

		case packetid.ClientboundLoginGameProfile:
			err := p.Scan(
				(*pk.UUID)(&c.UUID),
				(*pk.String)(&c.Name),
			)
			if err != nil {
				return LoginErr{"login success", err}
			}

			err = conn.WritePacket(pk.Marshal(packetid.ServerboundLoginLoginAcknowledged))
			if err != nil {
				return LoginErr{"login success", err}
			}

			return nil

		case packetid.ClientboundLoginLoginCompression:
			var threshold pk.VarInt
			err := p.Scan(&threshold)
			if err != nil {
				return LoginErr{"compression", err}
			}

			conn.SetThreshold(int(threshold))

			receiving = "login success"

		case packetid.ClientboundLoginCustomQuery:
			var (
				msgid   pk.VarInt
				channel pk.Identifier
				data    pk.PluginMessageData
			)

			err := p.Scan(&msgid, &channel, &data)
			if err != nil {
				return LoginErr{"Login Plugin", err}
			}

			var PluginMessageData pk.Option[pk.PluginMessageData, *pk.PluginMessageData]
			if handler, ok := c.LoginPlugin[string(channel)]; ok {
				PluginMessageData.Has = true
				PluginMessageData.Val, err = handler(data)
				if err != nil {
					return LoginErr{"Login Plugin", err}
				}
			}

			err = conn.WritePacket(pk.Marshal(
				packetid.ServerboundLoginCustomQueryAnswer,
				msgid, PluginMessageData,
			))
			if err != nil {
				return LoginErr{"login Plugin", err}
			}

		case packetid.ClientboundLoginCookieRequest:
			var key pk.Identifier
			err := p.Scan(&key)
			if err != nil {
				return LoginErr{"cookie request", err}
			}

			cookieContent := c.Cookies[string(key)]
			err = conn.WritePacket(pk.Marshal(
				packetid.ServerboundLoginCookieResponse,
				key, pk.OptionEncoder[pk.ByteArray]{
					Has: cookieContent != nil,
					Val: cookieContent,
				},
			))
			if err != nil {
				return LoginErr{"cookie response", err}
			}
		}
	}
}

type Auth struct {
	Name string
	UUID string
	AsTk string
}

func handleEncryptionRequest(conn *net.Conn, c *Client, p pk.Packet) error {
	key, encoStream, decoStream := newSymmetricEncryption()

	var er encryptionRequest
	if err := p.Scan(&er); err != nil {
		return err
	}

	err := loginAuth(c.Auth, key, er)
	if err != nil {
		return fmt.Errorf("login fail: %v", err)
	}

	p, err = genEncryptionKeyResponse(key, er.PublicKey, er.VerifyToken)
	if err != nil {
		return fmt.Errorf("gen encryption key response fail: %v", err)
	}

	err = conn.WritePacket(p)
	if err != nil {
		return err
	}

	conn.SetCipher(encoStream, decoStream)

	return nil
}

type encryptionRequest struct {
	ServerID    string
	PublicKey   []byte
	VerifyToken []byte
}

func (e *encryptionRequest) ReadFrom(r io.Reader) (int64, error) {
	return pk.Tuple{
		(*pk.String)(&e.ServerID),
		(*pk.ByteArray)(&e.PublicKey),
		(*pk.ByteArray)(&e.VerifyToken),
	}.ReadFrom(r)
}

func authDigest(serverID string, sharedSecret, publicKey []byte) string {
	h := sha1.New()
	h.Write([]byte(serverID))
	h.Write(sharedSecret)
	h.Write(publicKey)
	hash := h.Sum(nil)

	negative := (hash[0] & 0x80) == 0x80
	if negative {
		hash = twosComplement(hash)
	}

	res := strings.TrimLeft(hex.EncodeToString(hash), "0")
	if negative {
		res = "-" + res
	}

	return res
}

func twosComplement(p []byte) []byte {
	carry := true

	for i := len(p) - 1; i >= 0; i-- {
		p[i] = ^p[i]
		if carry {
			carry = p[i] == 0xff
			p[i]++
		}
	}

	return p
}

type profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type request struct {
	AccessToken     string  `json:"accessToken"`
	SelectedProfile profile `json:"selectedProfile"`
	ServerID        string  `json:"serverId"`
}

func loginAuth(auth Auth, shareSecret []byte, er encryptionRequest) error {
	digest := authDigest(er.ServerID, shareSecret, er.PublicKey)

	requestPacket, err := json.Marshal(
		request{
			AccessToken: auth.AsTk,
			SelectedProfile: profile{
				ID:   auth.UUID,
				Name: auth.Name,
			},
			ServerID: digest,
		},
	)
	if err != nil {
		return fmt.Errorf("create request packet to auth failed: %v", err)
	}

	PostRequest, err := http.NewRequest(http.MethodPost, "https://sessionserver.mojang.com/session/minecraft/join",
		bytes.NewReader(requestPacket))
	if err != nil {
		return fmt.Errorf("make request error: %v", err)
	}

	PostRequest.Header.Set("User-agent", "go-mc")
	PostRequest.Header.Set("Connection", "keep-alive")
	PostRequest.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(PostRequest)
	if err != nil {
		return fmt.Errorf("post fail: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("msauth fail: %s", string(body))
	}

	return nil
}

func newSymmetricEncryption() (key []byte, encoStream, decoStream cipher.Stream) {
	key = make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}

	b, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	decoStream = CFB8.NewCFB8Decrypt(b, key)
	encoStream = CFB8.NewCFB8Encrypt(b, key)

	return
}

func genEncryptionKeyResponse(shareSecret, publicKey, verifyToken []byte) (erp pk.Packet, err error) {
	iPK, err := x509.ParsePKIXPublicKey(publicKey)
	if err != nil {
		err = fmt.Errorf("decode public key fail: %v", err)

		return
	}

	rsaKey := iPK.(*rsa.PublicKey)
	cryptPK, err := rsa.EncryptPKCS1v15(rand.Reader, rsaKey, shareSecret)
	if err != nil {
		err = fmt.Errorf("encryption share secret fail: %v", err)

		return
	}

	verifyT, err := rsa.EncryptPKCS1v15(rand.Reader, rsaKey, verifyToken)
	if err != nil {
		err = fmt.Errorf("encryption verfy tokenfail: %v", err)

		return erp, err
	}

	return pk.Marshal(
		packetid.ServerboundLoginKey,
		pk.ByteArray(cryptPK),
		pk.ByteArray(verifyT),
	), nil
}
