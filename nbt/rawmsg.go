package nbt

import (
	"bytes"
	"io"
	"strings"
)

type RawMessage struct {
	Type byte
	Data []byte
}

func (m RawMessage) TagType() byte {
	return m.Type
}

func (m RawMessage) MarshalNBT(w io.Writer) error {
	_, err := w.Write(m.Data)

	return err
}

func (m *RawMessage) UnmarshalNBT(tagType byte, r DecoderReader) error {
	if tagType == TagEnd {
		return ErrEND
	}

	buf := bytes.NewBuffer(m.Data[:0])
	tee := io.TeeReader(r, buf)
	err := NewDecoder(tee).rawRead(tagType)
	if err != nil {
		return err
	}

	m.Type = tagType
	m.Data = buf.Bytes()

	return nil
}

func (m RawMessage) String() string {
	if m.Type == TagEnd {
		return ""
	}

	var stringMessage StringifiedMessage
	var stringBuilder strings.Builder
	stringReader := bytes.NewReader(m.Data)
	stringDecoder := NewDecoder(stringReader)
	err := stringMessage.encode(stringDecoder, &stringBuilder, m.Type)
	if err != nil {
		return "<Invalid: " + err.Error() + ">"
	}

	return stringBuilder.String()
}
