package inspection

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var ambiguousPower = regexp.MustCompile(`^[1-9]\d{0,2}\.\d{3}$`)
var decimalPower = regexp.MustCompile(`^\d+(?:[.,]\d{1,3})?$`)

// ParsePower uses the same decimal syntax and bounds as the browser inputs.
func ParsePower(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if ambiguousPower.MatchString(raw) {
		return 0, errors.New("Mehrdeutiger Punkt: Tausenderzeichen weglassen (4200). Dezimalstellen mit Komma eingeben")
	}
	if !decimalPower.MatchString(raw) {
		return 0, errors.New("Zahl mit höchstens drei Nachkommastellen eingeben (Komma oder Punkt)")
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
	if err != nil || value > 1000000 {
		return 0, errors.New("Leistung darf höchstens 1.000.000 W betragen")
	}
	return value, nil
}
