package main

import (
	"fmt"
	"strings"
	"time"
)

// DTCSeverity tells the service technician how urgent a fault is.
// These map loosely to AUTOSAR's DemEventStatusType levels.
type DTCSeverity int

const (
	SeverityInfo     DTCSeverity = iota // logged for visibility, no action needed
	SeverityWarning                     // driver should be notified, monitor closely
	SeverityCritical                    // take action now, may cause engine damage
)

func (s DTCSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARN"
	case SeverityCritical:
		return "CRIT"
	default:
		return "UNKN"
	}
}

// DTCEntry represents one logged fault event.
// In a real ECU these get written to non-volatile memory and read out
// by a scan tool over OBD-II. Here we keep them in a slice.
type DTCEntry struct {
	Code        string      // e.g. "P0217" - engine overheat
	SignalName  string      // which signal triggered it
	Value       float64     // the bad value that caused the fault
	Unit        string
	Description string
	Severity    DTCSeverity
	Timestamp   time.Time
}

// SignalSpec defines the valid operating range for one signal.
// Values outside [Min, Max] trip a fault and get a DTC logged.
type SignalSpec struct {
	Name        string
	Unit        string
	Min         float64     // below this is a fault
	Max         float64     // above this is a fault
	DTCCode     string      // the code logged when this signal faults
	Description string      // human-readable fault description
	Severity    DTCSeverity
}

// FaultLogger receives sensor readings, checks them against known-good ranges,
// and accumulates DTCs for any signal that goes out of bounds.
type FaultLogger struct {
	specs []SignalSpec
	log   []DTCEntry
}

// NewFaultLogger creates a logger preloaded with the signal specs we care about.
// The ranges here are realistic for a typical gasoline passenger vehicle.
func NewFaultLogger() *FaultLogger {
	return &FaultLogger{
		specs: []SignalSpec{
			{
				Name: "RPM", Unit: "rpm", Min: 500, Max: 7000,
				DTCCode:     "P0219",
				Description: "Engine overspeed detected",
				Severity:    SeverityCritical,
			},
			{
				Name: "COOLANT_TEMP", Unit: "C", Min: 40, Max: 115,
				DTCCode:     "P0217",
				Description: "Engine coolant temperature too high",
				Severity:    SeverityCritical,
			},
			{
				Name: "BATTERY_VOLT", Unit: "V", Min: 11.5, Max: 15.5,
				DTCCode:     "P0562",
				Description: "System voltage low",
				Severity:    SeverityWarning,
			},
			{
				Name: "OIL_PRESSURE", Unit: "bar", Min: 1.0, Max: 6.0,
				DTCCode:     "P0520",
				Description: "Oil pressure sensor out of range",
				Severity:    SeverityCritical,
			},
			{
				Name: "THROTTLE_POS", Unit: "%", Min: 0.0, Max: 100.0,
				DTCCode:     "P0122",
				Description: "Throttle position sensor low input",
				Severity:    SeverityWarning,
			},
			{
				Name: "VEHICLE_SPEED", Unit: "km/h", Min: 0, Max: 250,
				DTCCode:     "P0500",
				Description: "Vehicle speed sensor fault",
				Severity:    SeverityInfo,
			},
		},
	}
}

// Validate checks one reading against its spec. If the value is out of range,
// a DTC is logged and the function returns false. Returns true if the signal is healthy.
func (f *FaultLogger) Validate(name string, value float64, unit string) bool {
	for _, spec := range f.specs {
		if spec.Name != name {
			continue
		}

		if value < spec.Min || value > spec.Max {
			entry := DTCEntry{
				Code:        spec.DTCCode,
				SignalName:  name,
				Value:       value,
				Unit:        unit,
				Description: spec.Description,
				Severity:    spec.Severity,
				Timestamp:   time.Now(),
			}
			f.log = append(f.log, entry)
			return false
		}
		return true
	}

	// signal not in our spec list - log it as informational
	f.log = append(f.log, DTCEntry{
		Code:        "U0001",
		SignalName:  name,
		Value:       value,
		Unit:        unit,
		Description: "Unknown signal, no spec defined",
		Severity:    SeverityInfo,
		Timestamp:   time.Now(),
	})
	return false
}

// ValidateAll runs a batch of readings through the validator and prints a
// status line for each one. Returns the total number of faults found.
func (f *FaultLogger) ValidateAll(readings []SensorReading) int {
	faultCount := 0
	for _, r := range readings {
		ok := f.Validate(r.SignalName, r.Value, r.Unit)
		if ok {
			fmt.Printf("  [OK]   %-15s = %7.2f %-5s\n", r.SignalName, r.Value, r.Unit)
		} else {
			fmt.Printf("  [FAIL] %-15s = %7.2f %-5s  -> DTC logged\n", r.SignalName, r.Value, r.Unit)
			faultCount++
		}
	}
	return faultCount
}

// PrintDTCLog dumps all logged faults grouped by severity.
func (f *FaultLogger) PrintDTCLog() {
	if len(f.log) == 0 {
		fmt.Println("  No DTCs logged.")
		return
	}

	fmt.Printf("  %-6s  %-4s  %-15s  %8s  %s\n", "Time", "SEV", "Signal", "Value", "Description")
	fmt.Println("  " + strings.Repeat("-", 70))

	for _, entry := range f.log {
		fmt.Printf("  %s  [%s]  %-15s  %6.2f %-5s  %s (%s)\n",
			entry.Timestamp.Format("15:04:05.000"),
			entry.Severity,
			entry.SignalName,
			entry.Value,
			entry.Unit,
			entry.Description,
			entry.Code,
		)
	}

	fmt.Printf("\n  Total faults logged: %d\n", len(f.log))
}

// demonstrateSignalValidation runs a mix of healthy and faulty sensor readings
// through the fault logger to show how DTC detection works.
func demonstrateSignalValidation() {
	fmt.Println("\n========================================")
	fmt.Println("  Signal Validation & Fault Logger")
	fmt.Println("========================================")

	logger := NewFaultLogger()

	// mix of normal readings and intentionally bad values
	// the bad ones are labeled so it's obvious what's being tested
	readings := []SensorReading{
		{SignalName: "RPM",           Value: 3200,  Unit: "rpm"},   // normal
		{SignalName: "COOLANT_TEMP",  Value: 88.0,  Unit: "C"},     // normal
		{SignalName: "BATTERY_VOLT",  Value: 13.8,  Unit: "V"},     // normal
		{SignalName: "OIL_PRESSURE",  Value: 2.4,   Unit: "bar"},   // normal
		{SignalName: "THROTTLE_POS",  Value: 42.0,  Unit: "%"},     // normal
		{SignalName: "VEHICLE_SPEED", Value: 85.0,  Unit: "km/h"},  // normal
		{SignalName: "RPM",           Value: 7800,  Unit: "rpm"},   // FAULT: overspeed
		{SignalName: "COOLANT_TEMP",  Value: 118.0, Unit: "C"},     // FAULT: overtemp
		{SignalName: "BATTERY_VOLT",  Value: 10.9,  Unit: "V"},     // FAULT: low voltage
		{SignalName: "OIL_PRESSURE",  Value: 0.3,   Unit: "bar"},   // FAULT: low pressure
		{SignalName: "VEHICLE_SPEED", Value: 310.0, Unit: "km/h"},  // FAULT: over range
		{SignalName: "ENGINE_TORQUE", Value: 240.0, Unit: "Nm"},    // FAULT: unknown signal
	}

	fmt.Println("\n  Validating incoming sensor readings:")
	fmt.Println()
	faults := logger.ValidateAll(readings)

	fmt.Printf("\n  Result: %d/%d signals passed, %d fault(s) detected\n",
		len(readings)-faults, len(readings), faults)

	fmt.Println("\n  --- Active DTC Log ---")
	fmt.Println()
	logger.PrintDTCLog()
}
