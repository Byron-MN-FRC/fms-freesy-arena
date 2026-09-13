// Copyright 2026 Team 2175. All Rights Reserved.

package web

import (
	"encoding/json"
	"github.com/Team254/cheesy-arena/field"
	"github.com/Team254/cheesy-arena/plc"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// Sends a request that appears to originate from the given remote address, as an ESP32 module on the field would.
func (web *Web) httpResponseFrom(method, path, body, remoteAddr string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	web.newHandler().ServeHTTP(recorder, req)
	return recorder
}

func TestTeamStackLightFallsBackToRemoteAddress(t *testing.T) {
	web := setupTestWeb(t)
	web.arena.Esp32.SetRedAllianceStationEstopAddress("10.0.100.21")
	web.arena.Esp32.SetBlueAllianceStationEstopAddress("10.0.100.22")

	// Older firmware polls without the alliance parameter; the source IP identifies the module.
	recorder := web.httpResponseFrom("GET", "/api/freezy/team_stack_light", "", "10.0.100.22:40001")
	assert.Equal(t, 200, recorder.Code)
	assert.False(t, web.arena.Esp32.IsRedEstopsActive())
	assert.True(t, web.arena.Esp32.IsBlueEstopsActive())

	// An explicit alliance parameter wins over the source address.
	recorder = web.httpResponseFrom("GET", "/api/freezy/team_stack_light?alliance=red", "", "10.0.100.22:40002")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsRedEstopsActive())

	// An unknown source without the parameter leaves everything untouched.
	web.arena.Esp32 = newTestEsp32WithEstops()
	recorder = web.httpResponseFrom("GET", "/api/freezy/team_stack_light", "", "10.0.100.99:40003")
	assert.Equal(t, 200, recorder.Code)
	assert.False(t, web.arena.Esp32.IsRedEstopsActive())
	assert.False(t, web.arena.Esp32.IsBlueEstopsActive())
}

func TestHubStatusGetFallsBackToRemoteAddress(t *testing.T) {
	web := setupTestWeb(t)
	web.arena.Esp32.SetRedAllianceHubAddress("10.0.100.23")
	web.arena.Esp32.SetBlueAllianceHubAddress("10.0.100.24")

	recorder := web.httpResponseFrom("GET", "/api/freezy/hub_status", "", "10.0.100.23:40001")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsRedHubActive())
	assert.False(t, web.arena.Esp32.IsBlueHubActive())
}

func TestEstopAndStartAndCoilsEndpointsTrackActivity(t *testing.T) {
	web := setupTestWeb(t)
	web.arena.Esp32.SetScoreTableAddress("10.0.100.20")
	web.arena.Esp32.SetRedAllianceStationEstopAddress("10.0.100.21")
	web.arena.Esp32.SetBlueAllianceStationEstopAddress("10.0.100.22")

	// Blue estops box POSTs its stop states.
	recorder := web.httpResponseFrom("POST", "/api/freezy/eStopState", `[{"channel":0,"state":true}]`, "10.0.100.22:40001")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsBlueEstopsActive())
	assert.False(t, web.arena.Esp32.IsRedEstopsActive())
	assert.False(t, web.arena.Esp32.IsScoreTableActive())

	// Red estops box polls the coil map.
	recorder = web.httpResponseFrom("GET", "/api/freezy/alternateIO/PLC_Coils", "", "10.0.100.21:40002")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsRedEstopsActive())

	// Score table box presses start. The match won't actually start in the test arena, but the box is still alive.
	recorder = web.httpResponseFrom("POST", "/api/freezy/startMatch", "", "10.0.100.20:40003")
	assert.Equal(t, 200, recorder.Code)
	assert.True(t, web.arena.Esp32.IsScoreTableActive())
}

func newTestEsp32WithEstops() *plc.Esp32IO {
	esp32 := new(plc.Esp32IO)
	esp32.SetRedAllianceStationEstopAddress("10.0.100.21")
	esp32.SetBlueAllianceStationEstopAddress("10.0.100.22")
	return esp32
}

func (web *Web) postJsonHttpResponseFrom(path, remoteAddr, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	web.newHandler().ServeHTTP(recorder, req)
	return recorder
}

// The score table and each alliance station have their own field e-stop button, all sharing channel 0 and each
// reporting only its own contact. One device reporting "not pressed" must never clear another device's press.
func TestEStopStateFieldEStopIsWiredOr(t *testing.T) {
	web := setupTestWeb(t)
	web.arena.Esp32.SetScoreTableAddress("10.0.100.26")
	web.arena.Esp32.SetBlueAllianceStationEstopAddress("10.0.100.23")

	const scoreTable, blue = "10.0.100.26:55821", "10.0.100.23:61066"
	press := func(remoteAddr string, pressed bool) {
		body := `[{"channel":0,"state":true}]`
		if pressed {
			body = `[{"channel":0,"state":false}]`
		}
		assert.Equal(t, 200, web.postJsonHttpResponseFrom("/api/freezy/eStopState", remoteAddr, body).Code)
	}

	// Blue polling "not pressed" several times over does not clear the score table's press.
	press(blue, false)
	press(scoreTable, true)
	assert.True(t, web.arena.Plc.GetFieldEStop())
	press(blue, false)
	press(blue, false)
	assert.True(t, web.arena.Plc.GetFieldEStop())

	// It clears only once the score table itself reports released.
	press(scoreTable, false)
	assert.False(t, web.arena.Plc.GetFieldEStop())

	// Either button stops the field on its own.
	press(blue, true)
	assert.True(t, web.arena.Plc.GetFieldEStop())
	press(scoreTable, false)
	assert.True(t, web.arena.Plc.GetFieldEStop())

	// Both must be released before the field is clear.
	press(scoreTable, true)
	assert.True(t, web.arena.Plc.GetFieldEStop())
	press(blue, false)
	assert.True(t, web.arena.Plc.GetFieldEStop())
	press(scoreTable, false)
	assert.False(t, web.arena.Plc.GetFieldEStop())

	// The other stop channels are untouched by any of this; blue1EStop is channel 7.
	assert.Equal(t, 200, web.postJsonHttpResponseFrom(
		"/api/freezy/eStopState", blue, `[{"channel":7,"state":false}]`).Code)
	_, blueEStops := web.arena.Plc.GetTeamEStops()
	assert.True(t, blueEStops[0])
	assert.False(t, web.arena.Plc.GetFieldEStop())
}

// The field devices open a socket per request; the server closes it after responding so their pools do not fill.
func TestDeviceApiClosesConnection(t *testing.T) {
	web := setupTestWeb(t)
	for _, path := range []string{
		"/api/freezy/field_stack_light",
		"/api/freezy/team_stack_light",
		"/api/freezy/hub_status",
		"/api/freezy/alternateIO/PLC_Coils",
	} {
		recorder := web.getHttpResponse(path)
		assert.Equal(t, 200, recorder.Code, path)
		assert.Equal(t, "close", recorder.Header().Get("Connection"), path)
	}
	recorder := web.postJsonHttpResponseFrom("/api/freezy/eStopState", "10.0.100.26:1", `[{"channel":0,"state":true}]`)
	assert.Equal(t, 200, recorder.Code)
	assert.Equal(t, "close", recorder.Header().Get("Connection"))
}
