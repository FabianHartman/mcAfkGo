package chat

type HoverEvent struct {
	Action   string  `json:"action" nbt:"action"`
	Contents any     `json:"contents" nbt:"contents"`
	Value    Message `json:"value" nbt:"value"`
}
