package packet

import (
	"errors"
	"fmt"
	"io"
	"reflect"
)

type Ary[LEN VarInt | VarLong | Byte | UnsignedByte | Short | UnsignedShort | Int | Long] struct {
	Ary any `desc:"Slice or Pointer of Slice of FieldEncoder, FieldDecoder or both (Field)"`
}

func (a Ary[LEN]) WriteTo(w io.Writer) (n int64, err error) {
	array := reflect.ValueOf(a.Ary)
	for array.Kind() == reflect.Ptr {
		array = array.Elem()
	}

	Len := LEN(array.Len())
	nn, err := any(&Len).(FieldEncoder).WriteTo(w)
	if err != nil {
		return n, err
	} else {
		n += nn
	}
	for i := 0; i < array.Len(); i++ {
		elem := array.Index(i)
		nn, err := elem.Interface().(FieldEncoder).WriteTo(w)
		n += nn
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

func (a Ary[LEN]) ReadFrom(r io.Reader) (n int64, err error) {
	var Len LEN
	nn, err := any(&Len).(FieldDecoder).ReadFrom(r)
	if err != nil {
		return nn, err
	} else {
		n += nn
	}

	if Len < 0 {
		return n, errors.New("array length less than zero")
	}

	array := reflect.ValueOf(a.Ary)
	for array.Kind() == reflect.Ptr {
		array = array.Elem()
	}

	if !array.CanAddr() {
		panic(errors.New("the contents of the Ary are not addressable"))
	}
	if array.Cap() < int(Len) {
		array.Set(reflect.MakeSlice(array.Type(), int(Len), int(Len)))
	} else {
		array.Slice(0, int(Len))
	}

	for i := 0; i < int(Len); i++ {
		elem := array.Index(i)
		nn, err := elem.Addr().Interface().(FieldDecoder).ReadFrom(r)
		n += nn
		if err != nil {
			return n, err
		}
	}

	return n, err
}

func Array(ary any) Field {
	return Ary[VarInt]{Ary: ary}
}

type Opt struct {
	Has   any `desc:"Pointer of bool, or func() bool"`
	Field any `desc:"FieldEncoder, FieldDecoder, func() FieldEncoder, func() FieldDecoder or func() Field"`
}

func (o Opt) has() bool {
	v := reflect.ValueOf(o.Has)
	for {
		switch v.Kind() {
		case reflect.Ptr:
			v = v.Elem()
		case reflect.Bool:
			return v.Bool()
		case reflect.Func:
			return v.Interface().(func() bool)()
		default:
			panic(errors.New("unsupported Has value"))
		}
	}
}

func (o Opt) WriteTo(w io.Writer) (int64, error) {
	if o.has() {
		switch field := o.Field.(type) {
		case FieldEncoder:
			return field.WriteTo(w)
		case func() FieldEncoder:
			return field().WriteTo(w)
		case func() Field:
			return field().WriteTo(w)
		default:
			panic("unsupported Field type: " + reflect.TypeOf(o.Field).String())
		}
	}

	return 0, nil
}

func (o Opt) ReadFrom(r io.Reader) (int64, error) {
	if o.has() {
		switch field := o.Field.(type) {
		case FieldDecoder:
			return field.ReadFrom(r)
		case func() FieldDecoder:
			return field().ReadFrom(r)
		case func() Field:
			return field().ReadFrom(r)
		default:
			panic("unsupported Field type: " + reflect.TypeOf(o.Field).String())
		}
	}

	return 0, nil
}

type fieldPointer[T any] interface {
	*T
	FieldDecoder
}

type Option[T FieldEncoder, P fieldPointer[T]] struct {
	Has Boolean
	Val T
}

func (o Option[T, P]) WriteTo(w io.Writer) (n int64, err error) {
	n1, err := o.Has.WriteTo(w)
	if err != nil || !o.Has {
		return n1, err
	}

	n2, err := o.Val.WriteTo(w)

	return n1 + n2, err
}

func (o *Option[T, P]) ReadFrom(r io.Reader) (n int64, err error) {
	n1, err := o.Has.ReadFrom(r)
	if err != nil || !o.Has {
		return n1, err
	}

	n2, err := P(&o.Val).ReadFrom(r)

	return n1 + n2, err
}

type OptionDecoder[T any, P fieldPointer[T]] struct {
	Has Boolean
	Val T
}

func (o *OptionDecoder[T, P]) ReadFrom(r io.Reader) (n int64, err error) {
	n1, err := o.Has.ReadFrom(r)
	if err != nil || !o.Has {
		return n1, err
	}

	n2, err := P(&o.Val).ReadFrom(r)

	return n1 + n2, err
}

type OptionEncoder[T FieldEncoder] struct {
	Has Boolean
	Val T
}

func (o OptionEncoder[T]) WriteTo(w io.Writer) (n int64, err error) {
	n1, err := o.Has.WriteTo(w)
	if err != nil || !o.Has {
		return n1, err
	}

	n2, err := o.Val.WriteTo(w)

	return n1 + n2, err
}

type Tuple []any

func (t Tuple) WriteTo(w io.Writer) (n int64, err error) {
	for _, v := range t {
		nn, err := v.(FieldEncoder).WriteTo(w)
		if err != nil {
			return n, err
		}

		n += nn
	}

	return
}

func (t Tuple) ReadFrom(r io.Reader) (n int64, err error) {
	for i, v := range t {
		nn, err := v.(FieldDecoder).ReadFrom(r)
		if err != nil {
			return n, fmt.Errorf("decode tuple[%d] %T error: %w", i, v, err)
		}

		n += nn
	}

	return
}

func CreateByteReader(reader io.Reader) io.ByteReader {
	if byteReader, isByteReader := reader.(io.ByteReader); isByteReader {
		return byteReader
	}

	return byteReaderWrapper{reader}
}

type byteReaderWrapper struct {
	io.Reader
}

func (r byteReaderWrapper) ReadByte() (byte, error) {
	var buf [1]byte

	_, err := io.ReadFull(r.Reader, buf[:])

	return buf[0], err
}
