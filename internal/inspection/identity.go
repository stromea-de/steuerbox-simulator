package inspection

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewReportID creates 80 random bits independently of report contents and clocks.
func NewReportID() (string, error) {
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("Protokoll-ID nicht erzeugbar: %w", err)
	}
	s := base32.NewEncoding(crockford).WithPadding(base32.NoPadding).EncodeToString(raw[:])
	return s[:4] + "-" + s[4:8] + "-" + s[8:12] + "-" + s[12:], nil
}
func ValidReportID(s string) bool {
	if len(s) != 19 || s[4] != '-' || s[9] != '-' || s[14] != '-' {
		return false
	}
	for _, c := range strings.ReplaceAll(s, "-", "") {
		if !strings.ContainsRune(crockford, c) {
			return false
		}
	}
	return true
}
