package packet

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/google/uuid"

	"mcAfkGo/nbt"
)

type Field interface {
	FieldEncoder
	FieldDecoder
}

type FieldEncoder io.WriterTo

type FieldDecoder io.ReaderFrom

type (
	Boolean       bool
	Byte          int8
	UnsignedByte  uint8
	Short         int16
	UnsignedShort uint16
	Int           int32
	Long          int64
	Float         float32
	Double        float64
	String        string
	Identifier    = String
	VarInt        int32
	VarLong       int64
	Position      struct {
		X, Y, Z int
	}

	Angle             Byte
	UUID              uuid.UUID
	ByteArray         []byte
	PluginMessageData []byte
	BitSet            []int64
	FixedBitSet       []byte
)

const (
	MaxVarIntLen  = 5
	MaxVarLongLen = 10
)

func (b Boolean) WriteTo(w io.Writer) (int64, error) {
	var v byte
	if b {
		v = 0x01
	} else {
		v = 0x00
	}

	nn, err := w.Write([]byte{v})

	return int64(nn), err
}

func (b *Boolean) ReadFrom(r io.Reader) (n int64, err error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}

	*b = v != 0

	return n, nil
}

func (s String) WriteTo(w io.Writer) (int64, error) {
	byteStr := []byte(s)
	n1, err := VarInt(len(byteStr)).WriteTo(w)
	if err != nil {
		return n1, err
	}

	n2, err := w.Write(byteStr)

	return n1 + int64(n2), err
}

func (s *String) ReadFrom(r io.Reader) (n int64, err error) {
	var stringLength VarInt

	nn, err := stringLength.ReadFrom(r)
	if err != nil {
		return nn, err
	}
	n += nn

	bs := make([]byte, stringLength)
	_, err = io.ReadFull(r, bs)
	if err != nil {
		return n, err
	}

	n += int64(stringLength)

	*s = String(bs)
	return n, nil
}

func readByte(r io.Reader) (int64, byte, error) {
	byteReader, ok := r.(io.ByteReader)
	if ok {
		value, err := byteReader.ReadByte()

		return 1, value, err
	}

	var v [1]byte
	n, err := r.Read(v[:])

	return int64(n), v[0], err
}

func (b Byte) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write([]byte{byte(b)})

	return int64(nn), err
}

func (b *Byte) ReadFrom(r io.Reader) (n int64, err error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}

	*b = Byte(v)

	return n, nil
}

func (u UnsignedByte) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write([]byte{byte(u)})

	return int64(nn), err
}

func (u *UnsignedByte) ReadFrom(r io.Reader) (n int64, err error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}

	*u = UnsignedByte(v)

	return n, nil
}

func (s Short) WriteTo(w io.Writer) (int64, error) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(s))
	nn, err := w.Write(buf[:])

	return int64(nn), err
}

func (s *Short) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [2]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*s = Short(binary.BigEndian.Uint16(bs[:]))

	return
}

func (us UnsignedShort) WriteTo(w io.Writer) (int64, error) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(us))

	nn, err := w.Write(buf[:])

	return int64(nn), err
}

func (us *UnsignedShort) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [2]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*us = UnsignedShort(binary.BigEndian.Uint16(bs[:]))

	return
}

func (i Int) WriteTo(w io.Writer) (int64, error) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(i))

	nn, err := w.Write(buf[:])

	return int64(nn), err
}

func (i *Int) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [4]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*i = Int(binary.BigEndian.Uint32(bs[:]))

	return
}

func (l Long) WriteTo(w io.Writer) (int64, error) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(l))

	nn, err := w.Write(buf[:])

	return int64(nn), err
}

func (l *Long) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [8]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*l = Long(binary.BigEndian.Uint64(bs[:]))

	return
}

func (v VarInt) WriteTo(w io.Writer) (n int64, err error) {
	var vi [MaxVarIntLen]byte
	nn := v.WriteToBytes(vi[:])

	nn, err = w.Write(vi[:nn])

	return int64(nn), err
}

func (v VarInt) WriteToBytes(buf []byte) int {
	num := uint32(v)
	if num&0xFFFFFF80 == 0 {
		buf[0] = byte(num)

		return 1
	} else if num&0xFFFFC000 == 0 {
		result := uint16((num&0x7F|0x80)<<8 | (num >> 7))
		binary.BigEndian.PutUint16(buf, result)

		return 2
	} else if num&0xFFE00000 == 0 {
		buf[2] = byte(num >> 14)
		startingBytes := uint16((num&0x7F|0x80)<<8 | ((num>>
			7)&0x7F | 0x80))
		binary.BigEndian.PutUint16(buf, startingBytes)
		return 3
	} else if num&0xF0000000 == 0 {
		result := (num&0x7F|0x80)<<24 | (((num>>7)&0x7F | 0x80) << 16) |
			((num>>14)&0x7F|0x80)<<8 | (num >> 21)
		binary.BigEndian.PutUint32(buf, result)

		return 4
	} else {
		buf[4] = byte(num >> 28)
		startingBytes := (num&0x7F|0x80)<<24 | ((num>>7)&0x7F|0x80)<<16 |
			((num>>14)&0x7F|0x80)<<8 | ((num>>21)&0x7F | 0x80)
		binary.BigEndian.PutUint32(buf, startingBytes)

		return 5
	}
}

func (v *VarInt) ReadFrom(r io.Reader) (n int64, err error) {
	var V uint32
	var num int64
	byteReader := CreateByteReader(r)
	for sec := byte(0x80); sec&0x80 != 0; num++ {
		if num > MaxVarIntLen {
			return n, errors.New("VarInt is too big")
		}

		sec, err = byteReader.ReadByte()
		if err != nil {
			return n, err
		}

		n += 1

		V |= uint32(sec&0x7F) << uint32(7*num)
	}

	*v = VarInt(V)

	return
}

func (v VarInt) Len() int {
	switch {
	case v < 0:
		return MaxVarIntLen
	case v < 1<<(7*1):
		return 1
	case v < 1<<(7*2):
		return 2
	case v < 1<<(7*3):
		return 3
	case v < 1<<(7*4):
		return 4
	default:
		return 5
	}
}

func (v VarLong) WriteTo(w io.Writer) (n int64, err error) {
	var vi [MaxVarLongLen]byte
	nn := v.WriteToBytes(vi[:])
	nn, err = w.Write(vi[:nn])

	return int64(nn), err
}

func (v VarLong) WriteToBytes(buf []byte) int {
	num := uint64(v)
	n := v.Len()
	continuationBytes := n - 1
	_ = buf[continuationBytes]
	for i := 0; i < continuationBytes; i++ {
		buf[i] = byte(num&0x7F | 0x80)
		num >>= 7
	}

	buf[continuationBytes] = byte(num)

	return n
}

func (v *VarLong) ReadFrom(r io.Reader) (n int64, err error) {
	var V uint64
	var num int64
	byteReader := CreateByteReader(r)
	for sec := byte(0x80); sec&0x80 != 0; num++ {
		if num >= MaxVarLongLen {
			return n, errors.New("VarLong is too big")
		}

		sec, err = byteReader.ReadByte()
		if err != nil {
			return
		}

		n += 1

		V |= uint64(sec&0x7F) << uint64(7*num)
	}

	*v = VarLong(V)

	return
}

func (v VarLong) Len() int {
	switch {
	case v < 0:
		return MaxVarLongLen
	case v < 1<<(7*1):
		return 1
	case v < 1<<(7*2):
		return 2
	case v < 1<<(7*3):
		return 3
	case v < 1<<(7*4):
		return 4
	case v < 1<<(7*5):
		return 5
	case v < 1<<(7*6):
		return 6
	case v < 1<<(7*7):
		return 7
	case v < 1<<(7*8):
		return 8
	default:
		return 9
	}
}

func (p Position) WriteTo(w io.Writer) (n int64, err error) {
	var b [8]byte
	position := uint64(p.X&0x3FFFFFF)<<38 | uint64((p.Z&0x3FFFFFF)<<12) | uint64(p.Y&0xFFF)
	for i := 7; i >= 0; i-- {
		b[i] = byte(position)
		position >>= 8
	}

	nn, err := w.Write(b[:])

	return int64(nn), err
}

func (p *Position) ReadFrom(r io.Reader) (n int64, err error) {
	var v Long
	nn, err := v.ReadFrom(r)
	if err != nil {
		return nn, err
	}

	n += nn

	x := int(v >> 38)
	y := int(v << 52 >> 52)
	z := int(v << 26 >> 38)

	p.X, p.Y, p.Z = x, y, z

	return
}

func (a Angle) WriteTo(w io.Writer) (int64, error) {
	return Byte(a).WriteTo(w)
}

func (a *Angle) ReadFrom(r io.Reader) (int64, error) {
	return (*Byte)(a).ReadFrom(r)
}

func (f Float) WriteTo(w io.Writer) (n int64, err error) {
	return Int(math.Float32bits(float32(f))).WriteTo(w)
}

func (f *Float) ReadFrom(r io.Reader) (n int64, err error) {
	var v Int

	n, err = v.ReadFrom(r)
	if err != nil {
		return
	}

	*f = Float(math.Float32frombits(uint32(v)))
	return
}

func (d Double) WriteTo(w io.Writer) (n int64, err error) {
	return Long(math.Float64bits(float64(d))).WriteTo(w)
}

func (d *Double) ReadFrom(r io.Reader) (n int64, err error) {
	var v Long
	n, err = v.ReadFrom(r)
	if err != nil {
		return
	}

	*d = Double(math.Float64frombits(uint64(v)))

	return
}

func NBT(v any) Field {
	return NBTField{V: v}
}

type NBTField struct {
	V any

	AllowUnknownFields bool
}

func (n NBTField) WriteTo(w io.Writer) (int64, error) {
	if n.V == nil {
		n, err := w.Write([]byte{nbt.TagEnd})

		return int64(n), err
	}

	cw := countingWriter{w: w}
	enc := nbt.NewEncoder(&cw)
	enc.NetworkFormat(true)
	err := enc.Encode(n.V, "")

	return cw.n, err
}

func (n NBTField) ReadFrom(r io.Reader) (int64, error) {
	cr := countingReader{r: r}
	dec := nbt.NewDecoder(&cr)
	dec.NetworkFormat(true)
	if !n.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}

	_, err := dec.Decode(n.V)
	if err != nil {
		if !errors.Is(err, nbt.ErrEND) {
			return cr.n, err
		}
	}

	return cr.n, nil
}

type countingWriter struct {
	n int64
	w io.Writer
}

func (c *countingWriter) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	c.n += int64(n)

	return
}

type countingReader struct {
	n int64
	r io.Reader
}

func (c *countingReader) Read(p []byte) (n int, err error) {
	n, err = c.r.Read(p)
	c.n += int64(n)

	return
}

func (b ByteArray) WriteTo(w io.Writer) (n int64, err error) {
	n1, err := VarInt(len(b)).WriteTo(w)
	if err != nil {
		return n1, err
	}

	n2, err := w.Write(b)

	return n1 + int64(n2), err
}

func (b *ByteArray) ReadFrom(r io.Reader) (n int64, err error) {
	var Len VarInt
	n1, err := Len.ReadFrom(r)
	if err != nil {
		return n1, err
	}

	if cap(*b) < int(Len) {
		*b = make(ByteArray, Len)
	} else {
		*b = (*b)[:Len]
	}

	n2, err := io.ReadFull(r, *b)

	return n1 + int64(n2), err
}

func (u UUID) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write(u[:])

	return int64(nn), err
}

func (u *UUID) ReadFrom(r io.Reader) (n int64, err error) {
	nn, err := io.ReadFull(r, (*u)[:])

	return int64(nn), err
}

func (p PluginMessageData) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write(p)

	return int64(nn), err
}

func (p *PluginMessageData) ReadFrom(r io.Reader) (n int64, err error) {
	*p, err = io.ReadAll(r)

	return int64(len(*p)), err
}

func (b BitSet) WriteTo(w io.Writer) (n int64, err error) {
	n, err = VarInt(len(b)).WriteTo(w)
	if err != nil {
		return
	}

	for i := range b {
		n2, err := Long(b[i]).WriteTo(w)
		if err != nil {
			return n + n2, err
		}

		n += n2
	}

	return
}

func (b *BitSet) ReadFrom(r io.Reader) (n int64, err error) {
	var Len VarInt
	n, err = Len.ReadFrom(r)
	if err != nil {
		return
	}

	if int(Len) > cap(*b) {
		*b = make([]int64, Len)
	} else {
		*b = (*b)[:Len]
	}

	for i := 0; i < int(Len); i++ {
		n2, err := ((*Long)(&(*b)[i])).ReadFrom(r)
		if err != nil {
			return n + n2, err
		}

		n += n2
	}

	return
}

func (f FixedBitSet) WriteTo(w io.Writer) (n int64, err error) {
	n2, err := w.Write(f)

	return int64(n2), err
}

func (f FixedBitSet) ReadFrom(r io.Reader) (n int64, err error) {
	n2, err := r.Read(f)

	return int64(n2), err
}
