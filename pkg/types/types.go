package types

import "fmt"

type KeyType int

const (
	KeyTypeContent KeyType = iota
	KeyTypeSigning
	KeyTypeOTT
	KeyTypeOperatorSession
)

func (kt KeyType) String() string {
	names := []string{"CONTENT", "SIGNING", "OTT", "OPERATOR_SESSION"}
	if int(kt) < len(names) {
		return names[kt]
	}
	return "UNKNOWN"
}

type Key struct {
	KID         []byte
	Key         []byte
	Type        KeyType
	Permissions []string
}

func (k *Key) String() string {
	return fmt.Sprintf("[%s] %x:%x", k.Type, k.KID, k.Key)
}
