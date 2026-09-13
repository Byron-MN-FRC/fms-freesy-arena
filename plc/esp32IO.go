// Copyright 20## Team ###. All Rights Reserved.
// Author: cpapplefamily@gmail.com (Corey Applegate)
//
// Alternate IO handlers for the ###.

package plc

import (
	//"github.com/Team254/cheesy-arena/game"
	//"github.com/Team254/cheesy-arena/model"
	//"encoding/json"
	//"net/http"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Esp32 interface {
	Run()
	IsScoreTableIOEnabled() bool
	IsRedEstopsEnabled() bool
	IsBlueEstopsEnabled() bool
	IsRedHubEnabled() bool
	IsBlueHubEnabled() bool
	IsScoreTableHealthy() bool
	IsRedEstopsHealthy() bool
	IsBlueEstopsHealthy() bool
	IsRedHubHealthy() bool
	IsBlueHubHealthy() bool
	IsScoreTableActive() bool
	IsRedEstopsActive() bool
	IsBlueEstopsActive() bool
	IsRedHubActive() bool
	IsBlueHubActive() bool
	UpdateScoreTableLastSeen()
	UpdateRedEstopsLastSeen()
	UpdateBlueEstopsLastSeen()
	UpdateRedHubLastSeen()
	UpdateBlueHubLastSeen()
	UpdateLastSeenFromAddress(string) bool
	ModuleNameForAddress(string) string
	SetFieldEStopPressed(string, bool) (bool, bool)
	SetScoreTableAddress(string) 
	SetRedAllianceStationEstopAddress(string) 
	SetBlueAllianceStationEstopAddress(string) 
	SetRedAllianceHubAddress(string)
	SetBlueAllianceHubAddress(string)
	GetRedHubBatteryVoltage() float64
	GetRedHubBatteryPercent() float64
	GetBlueHubBatteryVoltage() float64
	GetBlueHubBatteryPercent() float64
	SetRedHubBattery(voltage, percent float64)
	SetBlueHubBattery(voltage, percent float64)
	SetPlc(Plc)
}

type Esp32IO struct {
	ScoreTableIP		string
	RedAllianceEstopsIP		string
	BlueAllianceEstopsIP		string
	RedAllianceHubIP		string
	BlueAllianceHubIP		string
	scoreTableHealthy 	bool
	RedEstopsHealthy 	bool
	BlueEstopsHealthy 	bool
	RedHubHealthy		bool
	BlueHubHealthy		bool
	// Timestamps for tracking when each module last called its API.
	ScoreTableLastSeen	time.Time
	RedEstopsLastSeen	time.Time
	BlueEstopsLastSeen	time.Time
	RedHubLastSeen		time.Time
	BlueHubLastSeen		time.Time
	// Hub battery status, as reported by the hub modules.
	RedHubBatteryVoltage	float64
	RedHubBatteryPercent	float64
	BlueHubBatteryVoltage	float64
	BlueHubBatteryPercent	float64
	// Field e-stop state most recently reported by each device that has a field e-stop button, keyed by host.
	fieldEStopPressed	map[string]bool
	fieldEStopMutex		sync.Mutex
	Plc Plc
}
const LoopPeriodMs = 1000 // Define the loop period in milliseconds

// How long to wait for a device to answer a probe. A configured device that is not plugged in never answers, so this
// is how long each probe of an absent device blocks the loop.
const deviceProbeTimeoutSec = 1



// RequestPayload represents the structure of the incoming POST data.
type RequestPayload struct {
	Channel int  `json:"channel"`
	State   bool `json:"state"`
}

func (esp32 *Esp32IO) SetScoreTableAddress(address string) {
	address = strings.TrimSpace(address)
	if address == "" {
		esp32.ScoreTableIP = address
        return
    }
    if net.ParseIP(address) == nil {
        log.Printf("Invalid Score Table IP address: %s", address)
        return
    }
    esp32.ScoreTableIP = address
    log.Printf("Set Score Table IP to: %s", esp32.ScoreTableIP)
}
func (esp32 *Esp32IO) SetRedAllianceStationEstopAddress(address string) {
	address = strings.TrimSpace(address)
	if address == "" {
		esp32.RedAllianceEstopsIP = address
        return
    }
    if net.ParseIP(address) == nil {
        log.Printf("Invalid Red Alliance Estops IP address: %s", address)
        return
    }
    esp32.RedAllianceEstopsIP = address
	log.Printf("Red Alliance Estops IP to: %s", esp32.RedAllianceEstopsIP)
}
func (esp32 *Esp32IO) SetBlueAllianceStationEstopAddress(address string) {
	address = strings.TrimSpace(address)
	if address == "" {
		esp32.BlueAllianceEstopsIP = address
        return
    }
    if net.ParseIP(address) == nil {
        log.Printf("Invalid Blue Alliance Estops IP address: %s", address)
        return
    }
    esp32.BlueAllianceEstopsIP = address
	log.Printf("Blue Alliance Estops IP to: %s", esp32.BlueAllianceEstopsIP)
}
func (esp32 *Esp32IO) SetRedAllianceHubAddress(address string) {
	address = strings.TrimSpace(address)
	if address == "" {
		esp32.RedAllianceHubIP = address
        return
    }
    if net.ParseIP(address) == nil {
        log.Printf("Invalid Red Alliance Hub IP address: %s", address)
        return
    }
    esp32.RedAllianceHubIP = address
	log.Printf("Red Alliance Hub IP to: %s", esp32.RedAllianceHubIP)
}
func (esp32 *Esp32IO) SetBlueAllianceHubAddress(address string) {
	address = strings.TrimSpace(address)
	if address == "" {
		esp32.BlueAllianceHubIP = address
        return
    }
    if net.ParseIP(address) == nil {
        log.Printf("Invalid Blue Alliance Hub IP address: %s", address)
        return
    }
    esp32.BlueAllianceHubIP = address
	log.Printf("Blue Alliance Hub IP to: %s", esp32.BlueAllianceHubIP)
}

func (esp32 *Esp32IO) SetPlc(plc Plc) {
	esp32.Plc = plc
}

// Checks if an IP address is reachable by attempting a TCP connection.
func isDevicePresent(ip string, port string) error {
    address := net.JoinHostPort(ip, port)
    conn, err := net.DialTimeout("tcp", address, time.Second*deviceProbeTimeoutSec)
    if err != nil {
        //log.Printf("Device not reachable at %s: %v", address, err)
        return err
    } 
    conn.Close()
    return err
}

// Run starts the ESP32 IO monitoring loop.
func (esp32 *Esp32IO) Run() {
	for {
		for _, device := range []struct {
			name    string
			address string
			healthy *bool
			active  func() bool
		}{
			{"Score Table", esp32.ScoreTableIP, &esp32.scoreTableHealthy, esp32.IsScoreTableActive},
			{"Red Estops", esp32.RedAllianceEstopsIP, &esp32.RedEstopsHealthy, esp32.IsRedEstopsActive},
			{"Blue Estops", esp32.BlueAllianceEstopsIP, &esp32.BlueEstopsHealthy, esp32.IsBlueEstopsActive},
			{"Red Hub", esp32.RedAllianceHubIP, &esp32.RedHubHealthy, esp32.IsRedHubActive},
			{"Blue Hub", esp32.BlueAllianceHubIP, &esp32.BlueHubHealthy, esp32.IsBlueHubActive},
		} {
			esp32.updateDeviceHealth(device.name, device.address, device.healthy, device.active)
		}

		esp32.Plc.ResetMatchReset()

		// Always pause a full period between passes rather than sleeping until a fixed deadline. Probing a device
		// that is configured but not plugged in blocks for the probe timeout, which overruns the deadline and would
		// otherwise leave no pause at all, dialing an absent host back to back and flooding the segment with ARP
		// traffic for it. That delays every other device on the field.
		time.Sleep(time.Millisecond * LoopPeriodMs)
	}
}

// Updates one device's health flag, logging only when it changes so that an absent device does not fill the log.
//
// A device that is calling our API is known to be up, so it is not probed. These are single-connection web servers,
// and a bare TCP connect that never sends a request can block one until its own HTTP timeout expires, delaying the
// button presses it is trying to report. Probing is only for devices that have gone quiet.
func (esp32 *Esp32IO) updateDeviceHealth(name, address string, healthy *bool, isActive func() bool) {
	if address == "" {
		// An empty address means the device is not configured.
		*healthy = false
		return
	}

	if !isActive() {
		if err := isDevicePresent(address, "80"); err != nil {
			if *healthy {
				log.Printf("%s not reachable at %s: %v", name, address, err)
			}
			*healthy = false
			return
		}
	}

	if !*healthy {
		log.Printf("%s Connected at: %s", name, address)
	}
	*healthy = true
}

// Returns whether the alternate IO is enabled.
func (esp32 *Esp32IO) IsScoreTableIOEnabled() bool {
	return esp32.ScoreTableIP != ""
}

// Returns whether the alternate IO is enabled.
func (esp32 *Esp32IO) IsRedEstopsEnabled() bool {
	return esp32.RedAllianceEstopsIP != ""
}

// Returns whether the alternate IO is enabled.
func (esp32 *Esp32IO) IsBlueEstopsEnabled() bool {
	return esp32.BlueAllianceEstopsIP != ""
}

// Returns the health status of the alternate IO.
func (esp32 *Esp32IO) IsScoreTableHealthy() bool {
	return esp32.scoreTableHealthy
}

// Returns the health status of the alternate IO.
func (esp32 *Esp32IO) IsRedEstopsHealthy() bool {
	return esp32.RedEstopsHealthy
}

// Returns the health status of the alternate IO.
func (esp32 *Esp32IO) IsBlueEstopsHealthy() bool {
	return esp32.BlueEstopsHealthy
}

// Returns whether the Red Alliance Hub is enabled.
func (esp32 *Esp32IO) IsRedHubEnabled() bool {
	return esp32.RedAllianceHubIP != ""
}

// Returns whether the Blue Alliance Hub is enabled.
func (esp32 *Esp32IO) IsBlueHubEnabled() bool {
	return esp32.BlueAllianceHubIP != ""
}

// Returns the health status of the Red Alliance Hub.
func (esp32 *Esp32IO) IsRedHubHealthy() bool {
	return esp32.RedHubHealthy
}

// Returns the health status of the Blue Alliance Hub.
func (esp32 *Esp32IO) IsBlueHubHealthy() bool {
	return esp32.BlueHubHealthy
}

// Activity timeout for determining if a module is still actively calling the API. Modules typically poll every
// second or two, so this needs enough slack to absorb wifi jitter without flapping the status badge.
const ModuleActivityTimeoutSec = 5

// Updates the last seen timestamp for the Score Table module.
func (esp32 *Esp32IO) UpdateScoreTableLastSeen() {
	esp32.ScoreTableLastSeen = time.Now()
}

// Updates the last seen timestamp for the Red Estops module.
func (esp32 *Esp32IO) UpdateRedEstopsLastSeen() {
	esp32.RedEstopsLastSeen = time.Now()
}

// Updates the last seen timestamp for the Blue Estops module.
func (esp32 *Esp32IO) UpdateBlueEstopsLastSeen() {
	esp32.BlueEstopsLastSeen = time.Now()
}

// Updates the last seen timestamp for the Red Hub module.
func (esp32 *Esp32IO) UpdateRedHubLastSeen() {
	esp32.RedHubLastSeen = time.Now()
}

// Updates the last seen timestamp for the Blue Hub module.
func (esp32 *Esp32IO) UpdateBlueHubLastSeen() {
	esp32.BlueHubLastSeen = time.Now()
}

// Updates the last seen timestamp for whichever module is configured at the given remote address (an "ip:port"
// string as found in http.Request.RemoteAddr, or a bare IP). Used as a fallback for API calls that don't identify
// the calling module explicitly. Returns whether the address matched a configured module.
func (esp32 *Esp32IO) UpdateLastSeenFromAddress(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	remoteIp := net.ParseIP(host)
	if remoteIp == nil {
		return false
	}

	matches := func(configured string) bool {
		if configured == "" {
			return false
		}
		return remoteIp.Equal(net.ParseIP(configured))
	}

	matched := false
	if matches(esp32.ScoreTableIP) {
		esp32.UpdateScoreTableLastSeen()
		matched = true
	}
	if matches(esp32.RedAllianceEstopsIP) {
		esp32.UpdateRedEstopsLastSeen()
		matched = true
	}
	if matches(esp32.BlueAllianceEstopsIP) {
		esp32.UpdateBlueEstopsLastSeen()
		matched = true
	}
	if matches(esp32.RedAllianceHubIP) {
		esp32.UpdateRedHubLastSeen()
		matched = true
	}
	if matches(esp32.BlueAllianceHubIP) {
		esp32.UpdateBlueHubLastSeen()
		matched = true
	}
	return matched
}

// Records the field e-stop state reported by the device at the given remote address, and returns whether any device
// currently reports it pressed along with whether this report changed that device's own state.
//
// The field e-stop is a wired-OR. The score table and each alliance station have their own button, and every device
// reports only its own contact, so one device reporting "not pressed" must never clear another device's press. The
// last device to report used to win, which let any device still polling clear an operator's stop within milliseconds.
func (esp32 *Esp32IO) SetFieldEStopPressed(remoteAddr string, pressed bool) (bool, bool) {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}

	esp32.fieldEStopMutex.Lock()
	defer esp32.fieldEStopMutex.Unlock()

	if esp32.fieldEStopPressed == nil {
		esp32.fieldEStopPressed = make(map[string]bool)
	}
	previous, seen := esp32.fieldEStopPressed[host]
	esp32.fieldEStopPressed[host] = pressed

	anyPressed := false
	for _, devicePressed := range esp32.fieldEStopPressed {
		if devicePressed {
			anyPressed = true
			break
		}
	}
	return anyPressed, !seen || previous != pressed
}

// Names of the configured modules, as reported by ModuleNameForAddress.
const (
	ModuleScoreTable = "score table"
	ModuleRedEstops  = "red alliance estops"
	ModuleBlueEstops = "blue alliance estops"
	ModuleRedHub     = "red alliance hub"
	ModuleBlueHub    = "blue alliance hub"
)

// Returns the name of the configured module at the given remote address (an "ip:port" string as found in
// http.Request.RemoteAddr, or a bare IP), or the empty string if no configured module is at that address.
func (esp32 *Esp32IO) ModuleNameForAddress(remoteAddr string) string {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	remoteIp := net.ParseIP(host)
	if remoteIp == nil {
		return ""
	}

	for _, module := range []struct {
		name       string
		configured string
	}{
		{ModuleScoreTable, esp32.ScoreTableIP},
		{ModuleRedEstops, esp32.RedAllianceEstopsIP},
		{ModuleBlueEstops, esp32.BlueAllianceEstopsIP},
		{ModuleRedHub, esp32.RedAllianceHubIP},
		{ModuleBlueHub, esp32.BlueAllianceHubIP},
	} {
		if module.configured != "" && remoteIp.Equal(net.ParseIP(module.configured)) {
			return module.name
		}
	}
	return ""
}

// Returns whether the Score Table module is actively calling the API.
func (esp32 *Esp32IO) IsScoreTableActive() bool {
	return time.Since(esp32.ScoreTableLastSeen).Seconds() < ModuleActivityTimeoutSec
}

// Returns whether the Red Estops module is actively calling the API.
func (esp32 *Esp32IO) IsRedEstopsActive() bool {
	return time.Since(esp32.RedEstopsLastSeen).Seconds() < ModuleActivityTimeoutSec
}

// Returns whether the Blue Estops module is actively calling the API.
func (esp32 *Esp32IO) IsBlueEstopsActive() bool {
	return time.Since(esp32.BlueEstopsLastSeen).Seconds() < ModuleActivityTimeoutSec
}

// Returns whether the Red Hub module is actively calling the API.
func (esp32 *Esp32IO) IsRedHubActive() bool {
	return time.Since(esp32.RedHubLastSeen).Seconds() < ModuleActivityTimeoutSec
}

// Returns whether the Blue Hub module is actively calling the API.
func (esp32 *Esp32IO) IsBlueHubActive() bool {
	return time.Since(esp32.BlueHubLastSeen).Seconds() < ModuleActivityTimeoutSec
}

// Returns the Red Hub battery voltage.
func (esp32 *Esp32IO) GetRedHubBatteryVoltage() float64 {
	return esp32.RedHubBatteryVoltage
}

// Returns the Red Hub battery percent.
func (esp32 *Esp32IO) GetRedHubBatteryPercent() float64 {
	return esp32.RedHubBatteryPercent
}

// Returns the Blue Hub battery voltage.
func (esp32 *Esp32IO) GetBlueHubBatteryVoltage() float64 {
	return esp32.BlueHubBatteryVoltage
}

// Returns the Blue Hub battery percent.
func (esp32 *Esp32IO) GetBlueHubBatteryPercent() float64 {
	return esp32.BlueHubBatteryPercent
}

// Sets the Red Hub battery status.
func (esp32 *Esp32IO) SetRedHubBattery(voltage, percent float64) {
	esp32.RedHubBatteryVoltage = voltage
	esp32.RedHubBatteryPercent = percent
}

// Sets the Blue Hub battery status.
func (esp32 *Esp32IO) SetBlueHubBattery(voltage, percent float64) {
	esp32.BlueHubBatteryVoltage = voltage
	esp32.BlueHubBatteryPercent = percent
}
