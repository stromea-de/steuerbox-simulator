package session

import (
	"context"
	"errors"
	"fmt"
	spineapi "github.com/enbility/spine-go/api"
	"time"

	"github.com/enbility/eebus-go/features/client"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func (s *Session) ReadFailsafe(ctx context.Context, k string) (inspection.FailsafeValues, error) {
	r := s.useCaseEntity(k)
	if r == nil {
		return inspection.FailsafeValues{}, errors.New("keine passende SPINE-Entität")
	}
	dc, err := client.NewDeviceConfiguration(s.EG.Entity, r)
	if err != nil {
		return inspection.FailsafeValues{}, err
	}
	powerName := model.DeviceConfigurationKeyNameTypeFailsafeConsumptionActivePowerLimit
	if k == "LPP" {
		powerName = model.DeviceConfigurationKeyNameTypeFailsafeProductionActivePowerLimit
	}
	names := []model.DeviceConfigurationKeyNameType{powerName, model.DeviceConfigurationKeyNameTypeFailsafeDurationMinimum}
	ids := make([]model.DeviceConfigurationKeyIdType, 2)
	for i, name := range names {
		desc, err := dc.GetKeyValueDescriptionsForFilter(model.DeviceConfigurationKeyValueDescriptionDataType{KeyName: util.Ptr(name)})
		if err != nil || len(desc) != 1 || desc[0].KeyId == nil {
			return inspection.FailsafeValues{}, errors.New("Failsafe-Beschreibung fehlt oder ist mehrdeutig")
		}
		ids[i] = *desc[0].KeyId
	}
	data, err := s.EG.ReadConfiguration(ctx, r)
	if err != nil {
		return inspection.FailsafeValues{}, err
	}
	var result inspection.FailsafeValues
	counts := [2]int{}
	for _, v := range data {
		if v.KeyId == nil || v.Value == nil {
			continue
		}
		if *v.KeyId == ids[0] && v.Value.ScaledNumber != nil && v.Value.ScaledNumber.Number != nil {
			result.Power = v.Value.ScaledNumber.GetValue()
			result.PowerNumber = util.Ptr(int64(*v.Value.ScaledNumber.Number))
			if v.Value.ScaledNumber.Scale != nil {
				result.PowerScale = util.Ptr(int8(*v.Value.ScaledNumber.Scale))
			}
			counts[0]++
		}
		if *v.KeyId == ids[1] && v.Value.Duration != nil {
			duration, err := v.Value.Duration.GetTimeDuration()
			if err != nil {
				return result, err
			}
			result.Minimum = duration
			result.MinimumRaw = string(*v.Value.Duration)
			counts[1]++
		}
	}
	if counts != [2]int{1, 1} {
		return result, errors.New("Failsafe-Ursprungswerte fehlen oder sind mehrdeutig")
	}
	result.Source = "SPINE reply: eigene Leseanfrage, Nachrichtenzuordnung und Gegenstelle geprüft; kein Cache"
	result.ReadAt = time.Now()
	return result, nil
}
func (s *Session) WriteFailsafe(ctx context.Context, k, field string, v inspection.FailsafeValues) error {
	if e := s.ControlError(); e != nil {
		return e
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r := s.useCaseEntity(k)
	if r == nil {
		return errors.New("keine passende SPINE-Entität")
	}
	dc, err := client.NewDeviceConfiguration(s.EG.Entity, r)
	if err != nil {
		return err
	}
	key := model.DeviceConfigurationKeyNameTypeFailsafeConsumptionActivePowerLimit
	if k == "LPP" {
		key = model.DeviceConfigurationKeyNameTypeFailsafeProductionActivePowerLimit
	}
	value := &model.DeviceConfigurationKeyValueValueType{}
	switch field {
	case "power":
		value.ScaledNumber = scaledWatts(v.Power)
		if v.PowerNumber != nil {
			value.ScaledNumber = &model.ScaledNumberType{Number: util.Ptr(model.NumberType(*v.PowerNumber))}
			if v.PowerScale != nil {
				value.ScaledNumber.Scale = util.Ptr(model.ScaleType(*v.PowerScale))
			}
		}
	case "minimum":
		key = model.DeviceConfigurationKeyNameTypeFailsafeDurationMinimum
		raw := model.NewDurationType(v.Minimum)
		if v.MinimumRaw != "" {
			raw = util.Ptr(model.DurationType(v.MinimumRaw))
		}
		value.Duration = raw
	default:
		return errors.New("unbekanntes Failsafe-Feld")
	}
	descriptions, err := dc.GetKeyValueDescriptionsForFilter(model.DeviceConfigurationKeyValueDescriptionDataType{KeyName: &key})
	if err != nil || len(descriptions) != 1 || descriptions[0].KeyId == nil {
		return errors.New("Failsafe-Beschreibung fehlt oder ist mehrdeutig")
	}
	result := make(chan model.ResultDataType, 1)
	var counter *model.MsgCounterType
	err = s.EG.WithControlRegistration(func() error {
		var sendErr error
		counter, sendErr = dc.WriteKeyValues([]model.DeviceConfigurationKeyValueDataType{{KeyId: descriptions[0].KeyId, Value: value}})
		if sendErr != nil {
			return sendErr
		}
		if counter == nil {
			return errors.New("keine Nachrichtennummer")
		}
		return dc.AddResponseCallback(*counter, func(msg spineapi.ResponseMessage) {
			if v, ok := msg.Data.(*model.ResultDataType); ok {
				select {
				case result <- *v:
				default:
				}
			}
		})
	})
	if err != nil {
		return err
	}
	return s.waitControlResult(ctx, *counter, result)
}

func (s *Session) configureDevice(ctx context.Context, c inspection.Config) error {
	caps, err := s.Capabilities(ctx)
	if err != nil {
		return err
	}
	for _, k := range []string{"LPC", "LPP"} {
		if !c.Includes(k) {
			continue
		}
		if k == "LPC" && !caps.LPC || k == "LPP" && !caps.LPP {
			continue
		}
		// R4-02: a function that appeared after the originals were read has no saved
		// originals; it is left alone instead of falling back to a free entity.
		if s.excluded(k) {
			continue
		}
		if _, err = s.Limit(ctx, k, 0, false); err != nil {
			return err
		}
		for _, field := range []string{"power", "minimum"} {
			if err = s.WriteFailsafe(ctx, k, field, inspection.FailsafeValues{Power: c.Failsafe, Minimum: c.FailsafeMinimum}); err != nil {
				return fmt.Errorf("%s Failsafe %s: %w", k, field, err)
			}
		}
	}
	return nil
}
