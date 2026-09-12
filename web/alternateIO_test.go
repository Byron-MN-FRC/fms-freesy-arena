// Copyright 2026 Team 2175. All Rights Reserved.

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Team254/cheesy-arena/field"
	"github.com/stretchr/testify/assert"
)

func (web *Web) postJsonHttpResponse(path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	web.newHandler().ServeHTTP(recorder, req)
	return recorder
}

func TestTeamHubStateGetHandler(t *testing.T) {
	web := setupTestWeb(t)

	web.arena.MatchState = field.PreMatch
	web.arena.FieldVolunteers = false
	recorder := web.getHttpResponse("/api/freezy/hub_status")
	assert.Equal(t, 200, recorder.Code)
	var states hubStates
	assert.Nil(t, json.Unmarshal(recorder.Body.Bytes(), &states))
	assert.Equal(t, field.HubDeviceLightState{Color: "purple"}, states.Red)
	assert.Equal(t, field.HubDeviceLightState{Color: "purple"}, states.Blue)

	web.arena.FieldVolunteers = true
	recorder = web.getHttpResponse("/api/freezy/hub_status")
	assert.Equal(t, 200, recorder.Code)
	assert.Nil(t, json.Unmarshal(recorder.Body.Bytes(), &states))
	assert.Equal(t, field.HubDeviceLightState{Color: "green"}, states.Red)
	assert.Equal(t, field.HubDeviceLightState{Color: "green"}, states.Blue)

	web.arena.MatchState = field.AutoPeriod
	recorder = web.getHttpResponse("/api/freezy/hub_status")
	assert.Equal(t, 200, recorder.Code)
	assert.Nil(t, json.Unmarshal(recorder.Body.Bytes(), &states))
	assert.Equal(t, field.HubDeviceLightState{Color: "red"}, states.Red)
	assert.Equal(t, field.HubDeviceLightState{Color: "blue"}, states.Blue)
}

func TestTeamHubStateGetHandlerTracksLastSeen(t *testing.T) {
	web := setupTestWeb(t)
	assert.False(t, web.arena.Esp32.IsRedHubActive())
	assert.False(t, web.arena.Esp32.IsBlueHubActive())

	recorder := web.getHttpResponse("/api/freezy/hub_status?alliance=red")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsRedHubActive())
	assert.False(t, web.arena.Esp32.IsBlueHubActive())

	recorder = web.getHttpResponse("/api/freezy/hub_status?alliance=b")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsBlueHubActive())
}

func TestTeamHubStatusPostHandler(t *testing.T) {
	web := setupTestWeb(t)

	recorder := web.postJsonHttpResponse("/api/freezy/hub_status?alliance=red", `{"voltage": 12.5, "percent": 85.0}`)
	assert.Equal(t, 200, recorder.Code)
	assert.Equal(t, 12.5, web.arena.Esp32.GetRedHubBatteryVoltage())
	assert.Equal(t, 85.0, web.arena.Esp32.GetRedHubBatteryPercent())
	assert.True(t, web.arena.Esp32.IsRedHubActive())

	recorder = web.postJsonHttpResponse("/api/freezy/hub_status?alliance=b", `{"voltage": 11.8, "percent": 72.5}`)
	assert.Equal(t, 200, recorder.Code)
	assert.Equal(t, 11.8, web.arena.Esp32.GetBlueHubBatteryVoltage())
	assert.Equal(t, 72.5, web.arena.Esp32.GetBlueHubBatteryPercent())
	assert.True(t, web.arena.Esp32.IsBlueHubActive())
}

func TestTeamHubStatusPostHandlerErrors(t *testing.T) {
	web := setupTestWeb(t)

	recorder := web.postJsonHttpResponse("/api/freezy/hub_status", `{"voltage": 12.5, "percent": 85.0}`)
	assert.Equal(t, 400, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "alliance")

	recorder = web.postJsonHttpResponse("/api/freezy/hub_status?alliance=green", `{"voltage": 12.5, "percent": 85.0}`)
	assert.Equal(t, 400, recorder.Code)

	recorder = web.postJsonHttpResponse("/api/freezy/hub_status?alliance=red", "invalid json")
	assert.Equal(t, 400, recorder.Code)
	assert.Equal(t, 0.0, web.arena.Esp32.GetRedHubBatteryVoltage())
}

func TestStackLightHandlersTrackLastSeen(t *testing.T) {
	web := setupTestWeb(t)
	assert.False(t, web.arena.Esp32.IsScoreTableActive())
	assert.False(t, web.arena.Esp32.IsRedEstopsActive())
	assert.False(t, web.arena.Esp32.IsBlueEstopsActive())

	recorder := web.getHttpResponse("/api/freezy/field_stack_light")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsScoreTableActive())

	recorder = web.getHttpResponse("/api/freezy/team_stack_light?alliance=red")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsRedEstopsActive())
	assert.False(t, web.arena.Esp32.IsBlueEstopsActive())

	recorder = web.getHttpResponse("/api/freezy/team_stack_light?alliance=blue")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsBlueEstopsActive())
}
