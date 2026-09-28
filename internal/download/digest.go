package download

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

func VerifyHexDigest(data []byte, expected string, algorithm string) error {
	var sum []byte
	switch algorithm {
	case "sha1":
		s := sha1.Sum(data)
		sum = s[:]
	case "sha256":
		s := sha256.Sum256(data)
		sum = s[:]
	default:
		return errors.New("unknown digest algorithm")
	}
	if !strings.EqualFold(hex.EncodeToString(sum), expected) {
		return fmt.Errorf("%s mismatch", algorithm)
	}
	return nil
}
