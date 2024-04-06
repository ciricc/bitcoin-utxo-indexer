package bigjson

import (
	"fmt"
	"math/big"
)

type BigFloat struct {
	big.Float
}

func NewBigFloat(f big.Float) *BigFloat {
	return &BigFloat{f}
}

func (b *BigFloat) MarshalJSON() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *BigFloat) String() string {
	s := b.Float.String()
	return s
}

func (b *BigFloat) UnmarshalJSON(p []byte) error {
	valueBytes := make([]byte, len(p))
	copy(valueBytes, p)

	if string(p) == "null" {
		return nil
	}

	var z big.Float
	_, ok := z.SetString(string(valueBytes))
	if !ok {
		return fmt.Errorf("not a valid big integer: %s", p)
	}

	b.Float = z
	return nil
}
