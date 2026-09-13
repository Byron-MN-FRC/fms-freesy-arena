// Copyright 2026 Team 2175. All Rights Reserved.

package field

import (
	"testing"
	"time"

	"github.com/Team254/cheesy-arena/game"
	"github.com/stretchr/testify/assert"
)

// Returns the offset from match start at which the given teleop shift begins.
func shiftStartOffset(shift game.Shift) time.Duration {
	teleopStart := time.Duration(game.MatchTiming.AutoDurationSec+game.MatchTiming.PauseDurationSec) * time.Second
	transition := time.Duration(game.MatchTiming.TransitionShiftDurationSec) * time.Second
	shiftLen := time.Duration(game.MatchTiming.ShiftDurationSec) * time.Second
	switch shift {
	case game.ShiftTransition:
		return teleopStart
	case game.Shift1, game.Shift2, game.Shift3, game.Shift4:
		return teleopStart + transition + time.Duration(shift-game.Shift1)*shiftLen
	case game.ShiftEndgame:
		return teleopStart + transition + 4*shiftLen
	}
	return 0
}

func setupHubDeviceTestArena(t *testing.T, redWonAuto bool) *Arena {
	arena := setupTestArena(t)
	arena.MatchState = TeleopPeriod
	arena.MatchStartTime = time.Unix(1_700_000_000, 0)
	arena.RedRealtimeScore.CurrentScore.Hub.WonAuto = redWonAuto
	arena.BlueRealtimeScore.CurrentScore.Hub.WonAuto = !redWonAuto
	return arena
}

func TestHubDeviceLightStatesOutsideMatch(t *testing.T) {
	arena := setupTestArena(t)
	now := time.Now()

	arena.MatchState = PreMatch
	arena.FieldVolunteers = false
	red, blue := arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "purple"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "purple"}, blue)

	arena.FieldVolunteers = true
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "green"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "green"}, blue)

	arena.MatchState = PostMatch
	arena.FieldVolunteers = false
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "purple"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "purple"}, blue)

	arena.MatchState = TimeoutActive
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, blue)
}

func TestHubDeviceLightStatesAutoAndPause(t *testing.T) {
	arena := setupTestArena(t)
	now := time.Now()

	for _, state := range []MatchState{AutoPeriod, PausePeriod} {
		arena.MatchState = state
		red, blue := arena.GetHubDeviceLightStates(now)
		assert.Equal(t, HubDeviceLightState{Color: "red"}, red)
		assert.Equal(t, HubDeviceLightState{Color: "blue"}, blue)
	}
}

func TestHubDeviceLightStatesTransition(t *testing.T) {
	// Both hubs are active during transition; the auto winner blinks because it goes inactive in shift 1.
	arena := setupHubDeviceTestArena(t, true)
	now := arena.MatchStartTime.Add(shiftStartOffset(game.ShiftTransition) + time.Second)
	red, blue := arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "red", Blink: true}, red)
	assert.Equal(t, HubDeviceLightState{Color: "blue", Blink: false}, blue)

	arena = setupHubDeviceTestArena(t, false)
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "red", Blink: false}, red)
	assert.Equal(t, HubDeviceLightState{Color: "blue", Blink: true}, blue)
}

func TestHubDeviceLightStatesShifts(t *testing.T) {
	arena := setupHubDeviceTestArena(t, true)
	shiftLen := time.Duration(game.MatchTiming.ShiftDurationSec) * time.Second
	warning := time.Duration(hubLightWarningSec) * time.Second

	// Shift 1: red won auto, so only blue is active.
	now := arena.MatchStartTime.Add(shiftStartOffset(game.Shift1) + time.Second)
	red, blue := arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "blue", Blink: false}, blue)

	// Blue blinks as shift 1 nears its end.
	now = arena.MatchStartTime.Add(shiftStartOffset(game.Shift1) + shiftLen - warning + time.Second)
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "blue", Blink: true}, blue)

	// Shift 2: only red is active.
	now = arena.MatchStartTime.Add(shiftStartOffset(game.Shift2) + time.Second)
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "red", Blink: false}, red)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, blue)

	// Shift 3: blue again.
	now = arena.MatchStartTime.Add(shiftStartOffset(game.Shift3) + time.Second)
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "blue", Blink: false}, blue)

	// Shift 4: red again.
	now = arena.MatchStartTime.Add(shiftStartOffset(game.Shift4) + time.Second)
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "red", Blink: false}, red)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, blue)
}

func TestHubDeviceLightStatesEndgame(t *testing.T) {
	arena := setupHubDeviceTestArena(t, true)
	endgameLen := time.Duration(game.MatchTiming.EndgameDurationSec) * time.Second
	warning := time.Duration(hubLightWarningSec) * time.Second

	// Both hubs are active in endgame.
	now := arena.MatchStartTime.Add(shiftStartOffset(game.ShiftEndgame) + time.Second)
	red, blue := arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "red", Blink: false}, red)
	assert.Equal(t, HubDeviceLightState{Color: "blue", Blink: false}, blue)

	// Both blink as the match nears its end.
	now = arena.MatchStartTime.Add(shiftStartOffset(game.ShiftEndgame) + endgameLen - warning + time.Second)
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "red", Blink: true}, red)
	assert.Equal(t, HubDeviceLightState{Color: "blue", Blink: true}, blue)

	// Past the end of teleop there is no valid shift, so the lights go off.
	now = arena.MatchStartTime.Add(shiftStartOffset(game.ShiftEndgame) + endgameLen + time.Second)
	red, blue = arena.GetHubDeviceLightStates(now)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, red)
	assert.Equal(t, HubDeviceLightState{Color: "black"}, blue)
}
