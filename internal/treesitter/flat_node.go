package treesitter

// FlatNode defines the 44-byte cache-aligned struct matching the C bridge layout.
type FlatNode struct {
	TypeID         uint16
	Flags          uint16
	StartByte      uint32
	EndByte        uint32
	StartRow       uint32
	StartCol       uint32
	EndRow         uint32
	EndCol         uint32
	ParentIdx      uint32
	FirstChildIdx  uint32
	NextSiblingIdx uint32
	ChildCount     uint32
}

const (
	FlatNodeNamed   = 1 << 0
	FlatNodeError   = 1 << 1
	FlatNodeMissing = 1 << 2
)
