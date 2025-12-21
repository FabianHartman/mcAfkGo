package basic

type Settings struct {
	Locale             string
	ViewDistance       int
	ChatMode           int
	ChatColors         bool
	DisplayedSkinParts uint8
	MainHand           int

	EnableTextFiltering bool
	AllowListing        bool

	Brand string
}

const (
	_ = 1 << iota
	Jacket
	LeftSleeve
	RightSleeve
	LeftPantsLeg
	RightPantsLeg
	Hat
)

var DefaultSettings = Settings{
	Locale:             "zh_CN", // ^_^
	ViewDistance:       15,
	ChatMode:           0,
	DisplayedSkinParts: Jacket | LeftSleeve | RightSleeve | LeftPantsLeg | RightPantsLeg | Hat,
	MainHand:           1,

	EnableTextFiltering: false,
	AllowListing:        true,

	Brand: "vanilla",
}
