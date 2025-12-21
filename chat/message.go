package chat

import (
	"fmt"
	"regexp"
	"strings"

	en_us "mcAfkGo/data/lang/en-us"
)

type Message struct {
	Text string `json:"text" nbt:"text"`

	Bold          bool `json:"bold,omitempty" nbt:"bold,omitempty"`                   // 粗体
	Italic        bool `json:"italic,omitempty" nbt:"italic,omitempty"`               // 斜体
	UnderLined    bool `json:"underlined,omitempty" nbt:"underlined,omitempty"`       // 下划线
	StrikeThrough bool `json:"strikethrough,omitempty" nbt:"strikethrough,omitempty"` // 删除线
	Obfuscated    bool `json:"obfuscated,omitempty" nbt:"obfuscated,omitempty"`       // 随机

	Font  string `json:"font,omitempty" nbt:"font,omitempty"`
	Color string `json:"color,omitempty" nbt:"color,omitempty"`

	Insertion  string      `json:"insertion,omitempty" nbt:"insertion,omitempty"`
	HoverEvent *HoverEvent `json:"hoverEvent,omitempty" nbt:"hoverEvent,omitempty"`

	Translate string        `json:"translate,omitempty" nbt:"translate,omitempty"`
	With      TranslateArgs `json:"with,omitempty" nbt:"with,omitempty"`
	Extra     []Message     `json:"extra,omitempty" nbt:"extra,omitempty"`
}

type TranslateArgs []any

type translateMsg struct {
	Text string `json:"text,omitempty" nbt:"text,omitempty"`

	Bold          bool `json:"bold,omitempty" nbt:"bold,omitempty"`
	Italic        bool `json:"italic,omitempty" nbt:"italic,omitempty"`
	UnderLined    bool `json:"underlined,omitempty" nbt:"underlined,omitempty"`
	StrikeThrough bool `json:"strikethrough,omitempty" nbt:"strikethrough,omitempty"`
	Obfuscated    bool `json:"obfuscated,omitempty" nbt:"obfuscated,omitempty"`

	Font  string `json:"font,omitempty" nbt:"font,omitempty"`
	Color string `json:"color,omitempty" nbt:"color,omitempty"`

	Insertion  string      `json:"insertion,omitempty" nbt:"insertion,omitempty"`
	HoverEvent *HoverEvent `json:"hoverEvent,omitempty" nbt:"hoverEvent,omitempty"`

	Translate string        `json:"translate,omitempty" nbt:"translate,omitempty"`
	With      TranslateArgs `json:"with,omitempty" nbt:"with,omitempty"`
	Extra     []Message     `json:"extra,omitempty" nbt:"extra,omitempty"`
}

type rawMsgStruct Message

func Text(str string) Message {
	return Message{Text: str}
}

var fmtCode = map[byte]string{
	'0': "30",
	'1': "34",
	'2': "32",
	'3': "36",
	'4': "31",
	'5': "35",
	'6': "33",
	'7': "37",
	'8': "90",
	'9': "94",
	'a': "92",
	'b': "96",
	'c': "91",
	'd': "95",
	'e': "93",
	'f': "97",

	'l': "1",
	'm': "9",
	'n': "4",
	'o': "3",
	'r': "0",
}

var translateMap = en_us.Map

func (m Message) ClearString() string {
	var msg strings.Builder
	text, _ := TransCtrlSeq(m.Text, false)
	msg.WriteString(text)

	if m.Translate != "" {
		args := make([]any, len(m.With))
		for i, v := range m.With {
			switch v := v.(type) {
			case Message:
				args[i] = v.ClearString()
			default:
				args[i] = v
			}
		}

		_, _ = fmt.Fprintf(&msg, translateMap[m.Translate], args...)
	}

	if m.Extra != nil {
		for i := range m.Extra {
			msg.WriteString(m.Extra[i].ClearString())
		}
	}
	return msg.String()
}

func (m Message) String() string {
	var msg, format strings.Builder
	if m.Bold {
		format.WriteString("1;")
	}
	if m.Italic {
		format.WriteString("3;")
	}
	if m.UnderLined {
		format.WriteString("4;")
	}
	if m.StrikeThrough {
		format.WriteString("9;")
	}
	if format.Len() > 0 {
		msg.WriteString("\033[" + format.String()[:format.Len()-1] + "m")
	}

	text, ok := TransCtrlSeq(m.Text, true)
	msg.WriteString(text)

	// handle translate
	if m.Translate != "" {
		_, _ = fmt.Fprintf(&msg, translateMap[m.Translate], m.With...)
	}

	if m.Extra != nil {
		for i := range m.Extra {
			msg.WriteString(m.Extra[i].String())
		}
	}

	if format.Len() > 0 || ok {
		msg.WriteString("\033[0m")
	}
	return msg.String()
}

var fmtPat = regexp.MustCompile(`(?i)§[\dA-FK-OR]`)

func TransCtrlSeq(str string, ansi bool) (dst string, change bool) {
	dst = fmtPat.ReplaceAllStringFunc(
		str,
		func(str string) string {
			f, ok := fmtCode[str[2]]
			if ok {
				if ansi {
					change = true
					return "\033[" + f + "m" // enable, add ANSI code
				}
				return "" // disable, remove the § code
			}
			return str // not a § code
		},
	)

	return
}
