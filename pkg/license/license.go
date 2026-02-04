package license

import "encoding/base64"

type License struct {
	Data []byte
	B64  string
}

func New(data string) *License {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return &License{
			Data: []byte(data),
			B64:  base64.StdEncoding.EncodeToString([]byte(data)),
		}
	}

	return &License{
		Data: decoded,
		B64:  data,
	}
}

func (l *License) Raw() []byte {
	return l.Data
}

func (l *License) Base64() string {
	return l.B64
}
