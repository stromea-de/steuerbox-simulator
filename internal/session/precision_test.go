package session

import (
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
	"testing"
)

func TestScaledWattsDoesNotTruncateValidatedDecimalInputs(t *testing.T) {
	for _, tc := range []struct {
		value  float64
		number int64
	}{{1.001, 1001}, {-1.001, -1001}, {4200.126, 4200126}, {-3000.126, -3000126}, {0.001, 1}, {1000000, 1000000000}} {
		encoded := scaledWatts(tc.value)
		if int64(*encoded.Number) != tc.number || *encoded.Scale != -3 {
			t.Fatal("changed wire decimal", tc, encoded)
		}
	}
	if !sameScaledWatts(&model.ScaledNumberType{Number: util.Ptr(model.NumberType(900)), Scale: util.Ptr(model.ScaleType(0))}, scaledWatts(900)) {
		t.Fatal("equivalent scaling rejected")
	}
}
