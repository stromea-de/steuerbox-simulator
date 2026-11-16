package inspection

import (
	"math"
	"testing"
)

func TestPowerParserAndAllNumericBoundaries(t *testing.T) {
	for _, raw := range []string{"4200,125", "4200.125"} {
		v, err := ParsePower(raw)
		if err != nil || v != 4200.125 {
			t.Fatal(raw, v, err)
		}
	}
	for _, raw := range []string{"NaN", "+Inf", "Infinity", "-1", "1.234,5", "1e6", "1.0001", "1000001", ""} {
		if _, err := ParsePower(raw); err == nil {
			t.Fatal(raw)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 1000001, 0.0001} {
		for field := 0; field < 4; field++ {
			c := testConfig()
			switch field {
			case 0:
				c.LPC = value
			case 1:
				c.LPP = value
			case 2:
				c.Failsafe = value
			case 3:
				c.Tolerance = value
			}
			if c.Validate() == nil {
				t.Fatal(field, value)
			}
		}
	}
}
