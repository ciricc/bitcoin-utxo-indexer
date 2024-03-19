package blockchain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
)

// Hash is a type for storing the hash of the block or transaction id
type Hash []byte

func (h Hash) String() string {
	return hex.EncodeToString(h)
}

func (h *Hash) MarshalJSON() ([]byte, error) {
	if h == nil {
		return []byte(`""`), nil
	}

	return json.Marshal(h.String())
}

func (h *Hash) UnmarshalJSON(v []byte) error {
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return err
	}

	if s == "" {
		return nil
	}

	hash, err := NewHashFromHEX(s)
	if err != nil {
		return fmt.Errorf("failed to unmarshal hash: %w", err)
	}

	hv := reflect.ValueOf(h).Elem()
	hv.SetBytes(hash)

	return nil
}

func MustHashFromHEX(encodedHash string) Hash {
	hash, err := NewHashFromHEX(encodedHash)
	if err != nil {
		panic(err)
	}

	return hash
}

func NewHashFromHEX(encodedHash string) (Hash, error) {
	hashBytes, err := hex.DecodeString(encodedHash)
	if err != nil {
		return nil, err
	}

	return hashBytes, nil
}
