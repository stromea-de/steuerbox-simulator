package eebus

import (
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
)

// MPC was constructed with no callback. Its measurement-data handler only
// computes callback notifications, but reads a payload that spine-go v0.7.0
// can mutate concurrently. Preserve all events needed for discovery and setup.
// If an MPC callback is ever introduced, this workaround must be revisited.
type mpcEventGuard struct {
	next spineapi.EventHandlerInterface
}

func (g *mpcEventGuard) HandleEvent(p spineapi.EventPayload) {
	if _, ok := p.Data.(*model.MeasurementListDataType); ok {
		return
	}
	g.next.HandleEvent(p)
}
