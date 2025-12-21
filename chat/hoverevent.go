package chat

// HoverEvent defines an event that occurs when this component hovered over.
type HoverEvent struct {
	Action   string  `json:"action" nbt:"action"`
	Contents any     `json:"contents" nbt:"contents"` // Didn't handled yet
	Value    Message `json:"value" nbt:"value"`       // Legacy
}
