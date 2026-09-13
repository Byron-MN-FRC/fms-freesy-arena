// Copyright 2026 Team 2175. All Rights Reserved.
//
// Light state for network-attached (ESP32) Hub devices, which mirror the state the PLC drives on the physical Hub
// lights but are reported over HTTP so that remote Hub modules can render it themselves.

package field

import (
	"time"

	"github.com/Team254/cheesy-arena/game"
)

// HubDeviceLightState is the color and blink state for a single alliance's Hub device.
type HubDeviceLightState struct {
	Color string `json:"color"`
	Blink bool   `json:"blink"`
}

// GetHubDeviceLightStates returns the light state for the red and blue Hub devices:
//
//	Alliance color, solid:   Hub is active
//	Alliance color, blinking: Hub is about to be deactivated
//	Purple:                  Field is safe for staff
//	Green:                   Field is safe for everyone
//	Black:                   Off
//
// The active/warning determination comes from game.Hub so that it stays consistent with the Hub lights the PLC
// drives in getHubLightStates.
func (arena *Arena) GetHubDeviceLightStates(currentTime time.Time) (HubDeviceLightState, HubDeviceLightState) {
	red := HubDeviceLightState{Color: "black"}
	blue := HubDeviceLightState{Color: "black"}

	switch arena.MatchState {
	case PreMatch, PostMatch:
		color := "purple"
		if arena.FieldVolunteers {
			color = "green"
		}
		red.Color = color
		blue.Color = color
	case AutoPeriod, PausePeriod:
		// Both Hubs are active for the whole of auto and the pause that follows it.
		red.Color = "red"
		blue.Color = "blue"
	case TeleopPeriod:
		redHub := &arena.RedRealtimeScore.CurrentScore.Hub
		blueHub := &arena.BlueRealtimeScore.CurrentScore.Hub
		shift, _, _, ok := redHub.GetCurrentShiftTiming(arena.MatchStartTime, currentTime)
		if !ok {
			return red, blue
		}

		if shift == game.ShiftTransition {
			// Both Hubs are active during the transition shift, but the alliance that won auto is about to go
			// inactive for shift 1.
			red.Color = "red"
			blue.Color = "blue"
			red.Blink = redHub.WonAuto
			blue.Blink = blueHub.WonAuto
			return red, blue
		}

		warningDuration := time.Duration(hubLightWarningSec) * time.Second
		if remaining, _ := redHub.GetActiveShiftTiming(arena.MatchStartTime, currentTime); remaining > 0 {
			red.Color = "red"
			red.Blink = remaining <= warningDuration
		}
		if remaining, _ := blueHub.GetActiveShiftTiming(arena.MatchStartTime, currentTime); remaining > 0 {
			blue.Color = "blue"
			blue.Blink = remaining <= warningDuration
		}
	}

	return red, blue
}
