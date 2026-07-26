package nbt

import (
	"io"
)

const (
	TagEnd byte = iota
	TagByte
	TagShort
	TagInt
	TagLong
	TagFloat
	TagDouble
	TagByteArray
	TagString
	TagList
	TagCompound
	TagIntArray
	TagLongArray
)

type DecoderReader = interface {
	io.ByteReader
	io.Reader
}
type Decoder struct {
	r                     DecoderReader
	disallowUnknownFields bool
	networkFormat         bool
}

func NewDecoder(r io.Reader) *Decoder {
	decoder := new(Decoder)
	decoderReader, ok := r.(DecoderReader)
	if ok {
		decoder.r = decoderReader
	} else {
		decoder.r = reader{r}
	}

	return decoder
}

func (d *Decoder) DisallowUnknownFields() {
	d.disallowUnknownFields = true
}

func (d *Decoder) NetworkFormat(enable bool) {
	d.networkFormat = enable
}

type reader struct {
	io.Reader
}

func (r reader) ReadByte() (byte, error) {
	var b [1]byte
	n, err := r.Read(b[:])
	if n == 1 {
		return b[0], nil
	}

	return 0, err
}
