package session

import (
	"errors"
	"math"
	"math/big"
	"time"

	"github.com/enbility/eebus-go/features/client"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
)

// scaledWatts preserves the validated milliwatt precision. spine-go v0.7.0's
// float helper truncates after multiplication (e.g. 1.001 W can become 1.000 W).
func scaledWatts(w float64) *model.ScaledNumberType {
	return &model.ScaledNumberType{Number: util.Ptr(model.NumberType(math.Round(w * 1000))), Scale: util.Ptr(model.ScaleType(-3))}
}
func (s *Session) writeLimit(r spineapi.EntityRemoteInterface, kind string, value float64, active bool, duration time.Duration, callback func(model.ResultDataType)) (*model.MsgCounterType, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1000000 {
		return nil, errors.New("ungültiger Grenzwert")
	}
	lc, err := client.NewLoadControl(s.EG.Entity, r)
	if err != nil {
		return nil, err
	}
	direction := model.EnergyDirectionTypeConsume
	if kind == "LPP" {
		direction = model.EnergyDirectionTypeProduce
	}
	limits, err := lc.GetLimitDataForFilter(model.LoadControlLimitDescriptionDataType{LimitType: util.Ptr(model.LoadControlLimitTypeTypeSignDependentAbsValueLimit), LimitDirection: &direction, ScopeType: util.Ptr(model.ScopeTypeTypeActivePowerLimit)})
	if err != nil || len(limits) != 1 || limits[0].LimitId == nil {
		return nil, errors.New("keine eindeutige Leistungsgrenze")
	}
	current := limits[0]
	if current.IsLimitChangeable != nil && !*current.IsLimitChangeable {
		return nil, errors.New("Leistungsgrenze nicht veränderbar")
	}
	data := model.LoadControlLimitDataType{LimitId: current.LimitId, Value: scaledWatts(value), IsLimitActive: &active, TimePeriod: &model.TimePeriodType{EndTime: model.NewAbsoluteOrRelativeTimeTypeFromDuration(duration)}}
	var counter *model.MsgCounterType
	err = s.EG.WithControlRegistration(func() error {
		var sendErr error
		counter, sendErr = lc.WriteLimitData([]model.LoadControlLimitDataType{data}, &model.LoadControlLimitListDataSelectorsType{LimitId: current.LimitId}, &model.LoadControlLimitDataElementsType{TimePeriod: &model.TimePeriodElementsType{}})
		if sendErr != nil {
			return sendErr
		}
		if counter == nil {
			return errors.New("keine Nachrichtennummer")
		}
		return lc.AddResponseCallback(*counter, func(msg spineapi.ResponseMessage) {
			if result, ok := msg.Data.(*model.ResultDataType); ok {
				callback(*result)
			}
		})
	})
	return counter, err
}

// Compare wire decimals as rationals, avoiding float rounding in GetValue().
func sameScaledWatts(a, b *model.ScaledNumberType) bool {
	if a == nil || b == nil || a.Number == nil || b.Number == nil {
		return false
	}
	rational := func(v *model.ScaledNumberType) *big.Rat {
		scale := int64(0)
		if v.Scale != nil {
			scale = int64(*v.Scale)
		}
		magnitude := scale
		if magnitude < 0 {
			magnitude = -magnitude
		}
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(magnitude), nil)
		r := new(big.Rat).SetInt64(int64(*v.Number))
		if scale < 0 {
			return r.Quo(r, new(big.Rat).SetInt(factor))
		}
		return r.Mul(r, new(big.Rat).SetInt(factor))
	}
	return rational(a).Cmp(rational(b)) == 0
}
