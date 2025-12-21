package user

import (
	"io"

	pk "mcAfkGo/net/packet"
)

type Property struct {
	Name, Value, Signature string
}

func (p Property) WriteTo(w io.Writer) (n int64, err error) {
	return pk.Tuple{
		pk.String(p.Name),
		pk.String(p.Value),
		pk.Option[pk.String, *pk.String]{
			Has: p.Signature != "",
			Val: pk.String(p.Signature),
		},
	}.WriteTo(w)
}

func (p *Property) ReadFrom(r io.Reader) (n int64, err error) {
	var signature pk.Option[pk.String, *pk.String]
	n, err = pk.Tuple{
		(*pk.String)(&p.Name),
		(*pk.String)(&p.Value),
		&signature,
	}.ReadFrom(r)
	p.Signature = string(signature.Val)
	return
}
