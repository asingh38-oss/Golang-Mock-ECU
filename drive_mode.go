package main

import (
	"fmt"
	"time"
)

// DriveMode represents the current operating mode of the vehicle.
// The ECU uses this to decide how aggressively to respond to throttle input,
// how strict to be about fault thresholds, and whether to restrict performance.
type DriveMode int

const (
	ModeNormal DriveMode = iota // standard everyday driving
	ModeEco                     // fuel-saving mode, throttle response softened
	ModeSport                   // performance mode, higher RPM limits allowed
	ModeLimp                    // fault fallback, restricts speed and RPM to protect the engine
)

// String gives us a readable label for each mode, used in logs and output.
func (m DriveMode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeEco:
		return "ECO"
	case ModeSport:
		return "SPORT"
	case ModeLimp:
		return "LIMP"
	default:
		return "UNKNOWN"
	}
}

// DriveModeConfig holds the threshold values that define when each mode activates.
// In a real AUTOSAR system these would come from calibration data in the .arxml.
type DriveModeConfig struct {
	LimpTempThreshold    float64 // coolant temp above this forces LIMP mode (C)
	LimpVoltThreshold    float64 // battery voltage below this forces LIMP mode (V)
	SportRPMThreshold    float64 // RPM above this suggests SPORT mode
	SportThrottleMin     float64 // throttle above this (%) combined with high RPM = SPORT
	EcoThrottleMax       float64 // throttle below this (%) at moderate RPM = ECO
	EcoRPMMax            float64 // RPM must also be below this for ECO to engage
}

// defaultDriveModeConfig returns a sensible set of thresholds for the simulator.
func defaultDriveModeConfig() DriveModeConfig {
	return DriveModeConfig{
		LimpTempThreshold: 100.0,
		LimpVoltThreshold: 11.8,
		SportRPMThreshold: 4500.0,
		SportThrottleMin:  60.0,
		EcoThrottleMax:    25.0,
		EcoRPMMax:         2500.0,
	}
}

// DriveModeController is the state machine that determines the active drive mode
// based on incoming sensor readings. It holds the current mode and the config
// used to evaluate transitions.
type DriveModeController struct {
	CurrentMode DriveMode
	Config      DriveModeConfig
	transitions int // count of how many mode changes have happened this session
}

// NewDriveModeController creates a controller starting in NORMAL mode.
func NewDriveModeController(cfg DriveModeConfig) *DriveModeController {
	return &DriveModeController{
		CurrentMode: ModeNormal,
		Config:      cfg,
	}
}

// Evaluate takes the latest sensor snapshot and returns the mode the ECU
// should be in right now. LIMP takes absolute priority - if any safety
// threshold is exceeded, nothing else matters.
//
// Transition logic (evaluated top to bottom, first match wins):
//   1. LIMP  - coolant too hot OR battery too low
//   2. SPORT - high RPM AND high throttle
//   3. ECO   - low throttle AND low RPM
//   4. NORMAL - everything else
func (c *DriveModeController) Evaluate(rpm, coolantTemp, batteryVolt, throttlePos float64) DriveMode {
	prev := c.CurrentMode
	cfg := c.Config

	var next DriveMode

	switch {
	case coolantTemp >= cfg.LimpTempThreshold || batteryVolt <= cfg.LimpVoltThreshold:
		next = ModeLimp

	case rpm >= cfg.SportRPMThreshold && throttlePos >= cfg.SportThrottleMin:
		next = ModeSport

	case throttlePos <= cfg.EcoThrottleMax && rpm <= cfg.EcoRPMMax:
		next = ModeEco

	default:
		next = ModeNormal
	}

	if next != prev {
		c.transitions++
	}
	c.CurrentMode = next
	return next
}

// demonstrateDriveMode runs the state machine through a scripted set of sensor
// snapshots that cover all four mode transitions. Each scenario is labeled so
// it's clear what condition is being tested.
func demonstrateDriveMode() {
	fmt.Println("\n========================================")
	fmt.Println("  Drive Mode State Machine")
	fmt.Println("========================================")

	cfg := defaultDriveModeConfig()
	ctrl := NewDriveModeController(cfg)

	fmt.Printf("\n  Thresholds:\n")
	fmt.Printf("    LIMP  -> coolant >= %.0fC  OR  battery <= %.1fV\n", cfg.LimpTempThreshold, cfg.LimpVoltThreshold)
	fmt.Printf("    SPORT -> RPM >= %.0f  AND  throttle >= %.0f%%\n", cfg.SportRPMThreshold, cfg.SportThrottleMin)
	fmt.Printf("    ECO   -> throttle <= %.0f%%  AND  RPM <= %.0f\n", cfg.EcoThrottleMax, cfg.EcoRPMMax)
	fmt.Println()

	// Each scenario represents a snapshot of sensor readings at a moment in time.
	// The controller evaluates all four signals and picks the correct mode.
	scenarios := []struct {
		label       string
		rpm         float64
		coolant     float64
		battery     float64
		throttle    float64
		expectMode  DriveMode
	}{
		{"Cold start, idle",                800,  72.0, 14.1, 5.0,  ModeEco},
		{"Steady highway cruise",           2200, 85.0, 13.8, 22.0, ModeEco},
		{"Moderate city driving",           2800, 87.0, 13.7, 35.0, ModeNormal},
		{"Hard acceleration on ramp",       5200, 89.0, 13.5, 78.0, ModeSport},
		{"Sustained high-rev driving",      5800, 94.0, 13.3, 65.0, ModeSport},
		{"Engine overheating",              3000, 102.0, 13.4, 40.0, ModeLimp},
		{"Battery fault (low voltage)",     2500, 88.0, 11.5, 30.0, ModeLimp},
		{"Recovery - temp back in range",   1800, 91.0, 13.9, 18.0, ModeEco},
	}

	fmt.Printf("  %-34s %-8s %-7s %-8s %-8s   %-8s %-8s\n",
		"Scenario", "RPM", "Temp C", "Batt V", "Thr %", "Mode", "Change?")
	fmt.Println("  " + repeatChar('-', 95))

	for _, s := range scenarios {
		before := ctrl.CurrentMode
		mode := ctrl.Evaluate(s.rpm, s.coolant, s.battery, s.throttle)
		changed := "  -"
		if mode != before {
			changed = fmt.Sprintf("  %s -> %s", before, mode)
		}
		fmt.Printf("  %-34s %6.0f  %5.1f   %5.1f    %5.1f      %-8s%s\n",
			s.label, s.rpm, s.coolant, s.battery, s.throttle, mode, changed)
		time.Sleep(10 * time.Millisecond) // small delay to make output feel like real-time
	}

	fmt.Printf("\n  Total mode transitions this session: %d\n", ctrl.transitions)
	fmt.Printf("  Final drive mode: %s\n", ctrl.CurrentMode)
}

// repeatChar is a small helper used for the table separator line.
func repeatChar(c rune, n int) string {
	s := make([]rune, n)
	for i := range s {
		s[i] = c
	}
	return string(s)
}
