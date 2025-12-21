package registry

import (
	"io"
	"reflect"

	pk "mcAfkGo/net/packet"
)

type Registries struct {
	// Empty - all registries are ignored as they're not needed by the bot
}

func NewNetworkCodec() Registries {
	return Registries{}
}

type RegistryCodec interface {
	pk.FieldDecoder
	ReadTagsFrom(r io.Reader) (int64, error)
}

func (c *Registries) Registry(id string) RegistryCodec {
	codecVal := reflect.ValueOf(c).Elem()
	codecTyp := codecVal.Type()
	numField := codecVal.NumField()
	for i := 0; i < numField; i++ {
		registryID, ok := codecTyp.Field(i).Tag.Lookup("registry")
		if !ok {
			continue
		}
		if registryID == id {
			return codecVal.Field(i).Addr().Interface().(RegistryCodec)
		}
	}
	return nil
}
