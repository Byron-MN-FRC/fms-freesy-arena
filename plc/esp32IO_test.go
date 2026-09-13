// Copyright 2026 Team 2175. All Rights Reserved.

package plc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEsp32IOUpdateLastSeenFromAddress(t *testing.T) {
	esp32 := new(Esp32IO)
	esp32.SetScoreTableAddress("10.0.100.20")
	esp32.SetRedAllianceStationEstopAddress("10.0.100.21")
	esp32.SetBlueAllianceStationEstopAddress("10.0.100.22")
	esp32.SetRedAllianceHubAddress("10.0.100.23")
	esp32.SetBlueAllianceHubAddress("10.0.100.24")

	testCases := []struct {
		name       string
		remoteAddr string
		matched    bool
		active     [5]bool // score table, red estops, blue estops, red hub, blue hub
	}{
		{"score table ip:port", "10.0.100.20:51234", true, [5]bool{true, false, false, false, false}},
		{"red estops bare ip", "10.0.100.21", true, [5]bool{false, true, false, false, false}},
		{"blue estops ip:port", "10.0.100.22:80", true, [5]bool{false, false, true, false, false}},
		{"red hub ipv4-mapped ipv6", "[::ffff:10.0.100.23]:1234", true, [5]bool{false, false, false, true, false}},
		{"blue hub", "10.0.100.24:9", true, [5]bool{false, false, false, false, true}},
		{"unknown host", "10.0.100.99:1234", false, [5]bool{}},
		{"garbage", "not-an-address", false, [5]bool{}},
		{"empty", "", false, [5]bool{}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			esp32 := *esp32 // Fresh timestamps for each case.
			assert.Equal(t, tc.matched, esp32.UpdateLastSeenFromAddress(tc.remoteAddr))
			assert.Equal(t, tc.active[0], esp32.IsScoreTableActive(), "score table")
			assert.Equal(t, tc.active[1], esp32.IsRedEstopsActive(), "red estops")
			assert.Equal(t, tc.active[2], esp32.IsBlueEstopsActive(), "blue estops")
			assert.Equal(t, tc.active[3], esp32.IsRedHubActive(), "red hub")
			assert.Equal(t, tc.active[4], esp32.IsBlueHubActive(), "blue hub")
		})
	}
}

func TestEsp32IOUpdateLastSeenFromAddressIgnoresUnconfiguredModules(t *testing.T) {
	// With no addresses configured, nothing should match even for an empty-string remote host.
	esp32 := new(Esp32IO)
	assert.False(t, esp32.UpdateLastSeenFromAddress("10.0.100.20:1234"))
	assert.False(t, esp32.IsScoreTableActive())
	assert.False(t, esp32.IsRedEstopsActive())
	assert.False(t, esp32.IsBlueEstopsActive())
}
