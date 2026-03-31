package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ECUConfig stores basic info about the ECU, things like its ID, name, and software version.
type ECUConfig struct {
	ECUID       int
	ECUName     string
	IsActive    bool
	SoftwareVer float64
}

// SensorReading holds a single snapshot from a sensor, including the signal name,
// its value, the unit it's measured in, and when it was captured.
type SensorReading struct {
	SignalName string
	Value      float64
	Unit       string
	Timestamp  time.Time
}

// Program 1 shows how an ECU works with different data types and the standard library
// methods that come up constantly in signal processing and diagnostics.
func demonstrateDataTypes() {
	fmt.Println("========================================")
	fmt.Println("  Program 1: Data Types & Built-in Methods")
	fmt.Println("========================================")

	// using RPM as a concrete example of integer signal values
	var rpmRaw int = 3450
	var rpmMax int = 8000
	fmt.Printf("\n[int] Raw RPM: %d | Max RPM: %d\n", rpmRaw, rpmMax)

	// strconv.Itoa converts an int to a string, useful when logging or formatting output
	rpmStr := strconv.Itoa(rpmRaw)
	fmt.Printf("  strconv.Itoa -> RPM as string: \"%s\"\n", rpmStr)

	// strconv.Atoi goes the other way, parsing a string back into an int
	parsedRPM, err := strconv.Atoi("5200")
	if err == nil {
		fmt.Printf("  strconv.Atoi -> Parsed RPM from string: %d\n", parsedRPM)
	}

	// float64 is the natural fit for analog readings like voltage and temperature
	var voltage float64 = 13.756
	var tempCelsius float64 = 87.345
	fmt.Printf("\n[float64] Battery Voltage: %.3f V | Engine Temp: %.3f C\n", voltage, tempCelsius)

	// math.Round cleans up noisy sensor values before logging or comparing
	fmt.Printf("  math.Round  -> Voltage rounded: %.1f V\n", math.Round(voltage*10)/10)

	// math.Abs gives us the magnitude of deviation from a nominal value
	nominalVoltage := 14.4
	delta := math.Abs(nominalVoltage - voltage)
	fmt.Printf("  math.Abs    -> Voltage delta from nominal (%.1fV): %.3f V\n", nominalVoltage, delta)

	// signal names coming from different ECUs often have inconsistent formatting
	var signalName string = "  engine_speed_rpm  "
	var ecuLabel string = "POWERTRAIN_ECU_v2"
	fmt.Printf("\n[string] Raw Signal Name: \"%s\"\n", signalName)

	// TrimSpace strips the leading and trailing whitespace before we do anything with it
	trimmed := strings.TrimSpace(signalName)
	fmt.Printf("  strings.TrimSpace  -> \"%s\"\n", trimmed)

	// normalize the signal name to a standard display format
	formatted := strings.ToUpper(strings.Replace(trimmed, "_", ".", -1))
	fmt.Printf("  strings.ToUpper + strings.Replace -> \"%s\"\n", formatted)

	fmt.Printf("  strings.Contains (\"POWERTRAIN\" in label): %v\n", strings.Contains(ecuLabel, "POWERTRAIN"))

	// booleans represent fault flags and mode states throughout the ECU
	var dtcActive bool = false
	var limpModeEnabled bool = tempCelsius > 85.0
	fmt.Printf("\n[bool] DTC Active: %v | Limp Mode Enabled: %v\n", dtcActive, limpModeEnabled)

	// format both flags into a single status string the way a real DTC log might look
	dtcStatus := fmt.Sprintf("DTC_STATUS=%s | LIMP_MODE=%s",
		strings.ToUpper(strconv.FormatBool(dtcActive)),
		strings.ToUpper(strconv.FormatBool(limpModeEnabled)),
	)
	fmt.Printf("  fmt.Sprintf -> \"%s\"\n", dtcStatus)
}

// Program 2 shows how Go's data structures map onto common ECU patterns
// like signal tables, threshold maps, and component configuration structs.
func demonstrateDataStructures() {
	fmt.Println("\n========================================")
	fmt.Println("  Program 2: Data Structures & Control Structures")
	fmt.Println("========================================")

	// arrays work well for fixed sets like CAN bus IDs, which don't change at runtime
	var canBusIDs [4]int = [4]int{0x100, 0x200, 0x300, 0x400}
	fmt.Println("\n[Array] CAN Bus IDs:")
	for i, id := range canBusIDs {
		fmt.Printf("  canBusIDs[%d] = 0x%X\n", i, id)
	}

	// slices are more flexible and work better for sensor readings that grow over time
	readings := []SensorReading{
		{SignalName: "THROTTLE_POS", Value: 45.2, Unit: "%"},
		{SignalName: "COOLANT_TEMP", Value: 92.1, Unit: "C"},
		{SignalName: "BATTERY_VOLT", Value: 13.8, Unit: "V"},
		{SignalName: "VEHICLE_SPEED", Value: 78.5, Unit: "km/h"},
		{SignalName: "OIL_PRESSURE", Value: 2.1, Unit: "bar"},
	}

	fmt.Println("\n[Slice] PDU Signal Readings:")
	var total float64
	for _, r := range readings {
		total += r.Value
	}
	avg := total / float64(len(readings))
	fmt.Printf("  Total signals: %d | Avg value: %.2f\n", len(readings), avg)

	// a map is a natural fit for threshold tables since we look up by signal name
	fmt.Println("\n[if-else] Fault Detection:")
	faultThresholds := map[string]float64{
		"COOLANT_TEMP": 90.0,
		"BATTERY_VOLT": 11.5,
		"OIL_PRESSURE": 1.5,
	}

	for _, r := range readings {
		threshold, monitored := faultThresholds[r.SignalName]
		if monitored {
			if r.Value > threshold && r.SignalName == "COOLANT_TEMP" {
				fmt.Printf("  FAULT: %s = %.1f %s (exceeds %.1f)\n", r.SignalName, r.Value, r.Unit, threshold)
			} else if r.Value < threshold {
				fmt.Printf("  FAULT: %s = %.1f %s (below %.1f)\n", r.SignalName, r.Value, r.Unit, threshold)
			} else {
				fmt.Printf("  OK:    %s = %.1f %s\n", r.SignalName, r.Value, r.Unit)
			}
		}
	}

	// structs are how we represent ECU components with multiple fields of different types
	ecu := ECUConfig{
		ECUID:       1,
		ECUName:     "POWERTRAIN_ECU",
		IsActive:    true,
		SoftwareVer: 2.41,
	}
	fmt.Printf("\n[Struct] ECU -> ID: %d | Name: %s | Active: %v | SW Ver: %.2f\n",
		ecu.ECUID, ecu.ECUName, ecu.IsActive, ecu.SoftwareVer)
}

// simulateSensor runs as a goroutine and generates random readings for a single sensor.
// Each call to this represents one independent sensor node running on its own schedule.
func simulateSensor(name string, unit string, min, max float64, ch chan<- SensorReading, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 3; i++ {
		value := min + rand.Float64()*(max-min)
		reading := SensorReading{
			SignalName: name,
			Value:      math.Round(value*100) / 100,
			Unit:       unit,
			Timestamp:  time.Now(),
		}
		ch <- reading
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)+50))
	}
}

// Program 3 demonstrates concurrency using goroutines and channels, which maps well
// onto how an AUTOSAR RTE schedules parallel runnable tasks across software components.
func demonstrateConcurrency() {
	fmt.Println("\n========================================")
	fmt.Println("  Program 3: Concurrency & Exception Handling")
	fmt.Println("========================================")

	ch := make(chan SensorReading, 20)
	var wg sync.WaitGroup

	sensors := []struct {
		name     string
		unit     string
		min, max float64
	}{
		{"RPM", "rpm", 800, 6500},
		{"COOLANT_TEMP", "C", 70, 105},
		{"BATTERY_VOLT", "V", 12.0, 14.8},
		{"VEHICLE_SPEED", "km/h", 0, 120},
	}

	// each sensor gets its own goroutine so they all run independently
	fmt.Println("\n[Goroutines] Starting sensor streams...")
	for _, s := range sensors {
		wg.Add(1)
		go simulateSensor(s.name, s.unit, s.min, s.max, ch, &wg)
	}

	// close the channel once all sensor goroutines have finished sending
	go func() {
		wg.Wait()
		close(ch)
	}()

	// drain the channel as readings come in, this blocks until the channel is closed
	fmt.Println("\n[Channel] Incoming readings:")
	count := 0
	for r := range ch {
		fmt.Printf("  [%s] %-15s = %.2f %s\n",
			r.Timestamp.Format("15:04:05.000"), r.SignalName, r.Value, r.Unit)
		count++
	}
	fmt.Printf("\n  Total readings: %d from %d sensors\n", count, len(sensors))

	// panic/recover lets the ECU catch runtime errors and enter a safe fallback state
	// instead of crashing the whole process
	fmt.Println("\n[Panic/Recover] Testing ECU fault recovery:")
	safeECUOperation(0)   // passes zero on purpose to trigger a divide by zero panic
	safeECUOperation(250) // normal case, should complete without issues
}

// safeECUOperation wraps a potentially dangerous calculation in a recover block.
// If anything panics inside, it gets caught and the ECU logs a fault instead of crashing.
func safeECUOperation(loadValue int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("  PANIC RECOVERED: %v - entering safe mode\n", r)
		}
	}()
	result := 1000 / loadValue
	fmt.Printf("  ECU load calc: 1000 / %d = %d\n", loadValue, result)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║     AUTOSAR Mock ECU Simulator - Go      ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	demonstrateDataTypes()
	demonstrateDataStructures()
	demonstrateConcurrency()
	demonstrateCANLayer()

	fmt.Println("\n========================================")
	fmt.Println("  Done")
	fmt.Println("========================================")
}