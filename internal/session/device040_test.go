package session

import (
	"testing"

	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// 040 F-11: with several classified entities the first (device-information) entity wins;
// a sub-component reported later must not replace serial number or firmware.
func TestManufacturerFirstEntityWins040(t *testing.T) {
	text := func(v string) *model.DeviceClassificationStringType {
		x := model.DeviceClassificationStringType(v)
		return &x
	}
	main := model.DeviceClassificationManufacturerDataType{VendorName: text("Hersteller"), SerialNumber: text("SN-GERAET"), SoftwareRevision: text("1.2.3"), DeviceName: text("Wärmepumpe X")}
	sub := model.DeviceClassificationManufacturerDataType{BrandName: text("Zulieferer"), SerialNumber: text("SN-MODUL"), SoftwareRevision: text("9.9")}
	d := applyManufacturer(inspection.Device{SKI: "abc", Brand: "aus mDNS"}, []model.DeviceClassificationManufacturerDataType{main, sub})
	if d.Brand != "Hersteller" || d.Serial != "SN-GERAET" || d.Firmware != "1.2.3" || d.Model != "Wärmepumpe X" {
		t.Fatalf("%+v", d)
	}
}
