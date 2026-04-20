package main

import (
	"fmt"
	"strings"
	"time"
)

// OBD 2 Service tool for ID identification
// a real scan tool sends an ID to the ECU thru CAN transport and the ECU proceeds to respond with requested data, 
// This is a simulation of the same send/request response structure just in the memory as opposed to a physical can bus.


const (
	ServiceShowCurrentData  = 0x01 // live PID values
	ServiceShowFreezeFrame  = 0x02 // sensor snapshot captured at fault time
	ServiceShowStoredDTCs   = 0x03 // read all stored fault codes
	ServiceClearDTCs        = 0x04 // erase fault codes and freeze frame
	ServiceShowPendingDTCs  = 0x07 // faults detected but not yet confirmed
	ServiceRequestVehicleInfo = 0x09 // VIN, calibration IDs, etc.
)


// Pid constants for service variable 01

const(
	PID_SUPPORTED_PIDS    = 0x00 // bitmask of which PIDs we support
	PID_ENGINE_RPM        = 0x0C // engine speed in 0.25 RPM steps
	PID_VEHICLE_SPEED     = 0x0D // km/h, one byte, 0-255
	PID_COOLANT_TEMP      = 0x05 // temp in C + 40 offset, one byte
	PID_THROTTLE_POSITION = 0x11 // 0-100%, scaled to one byte
	PID_BATTERY_VOLTAGE   = 0x42 // control module voltage, two bytes, 0.001V/bit
	PID_OIL_PRESSURE      = 0x52 // oil pressure placeholder (manufacturer-defined range)
	PID_ENGINE_LOAD       = 0x04 // calculated engine load 0-100%, one byte
	PID_VIN               = 0x02 // 17-character VIN (used with Service 09)
)

// OBD request will represent a message from a scan tool to the ECU, this will travel inside the mock CAN frame.

type OBDRequest struct {
	ServiceID byte
	PID byte   // PID will be relevant for service variable 01, 02, 09
	Timestamp time.Time
}

// OBD response will be what the ECU sends back. The payload is received as raw bytes,
// the decoded value you receive is the human readable version for the console output.
type OBDResponse struct {
	ServiceID    byte
	PID          byte
	RawBytes     []byte
	DecodedValue string
	Unit         string
	Timestamp    time.Time
}


// Freezeframe will capture a full sensor snapshot at the moment a DTC is logged
// A real ECU will store one freeze frame per DTC in a non volatile memory 
// for this use case we will store them in a slice since we dont have a NVM
type FreezeFrame struct {
	DTCCode  string
	Readings []SensorReading
	CapturedAt time.Time
}

// OBDInterface is the simulated diagnostic port on the ECU.
type OBDInterface struct {
	vin         string
	faultLogger *FaultLogger    // shared with the rest of the simulator
	liveData    map[string]SensorReading // latest readings, keyed by signal name
	freezeFrames []FreezeFrame
	pendingDTCs []string        // faults seen once but not yet confirmed (two-drive cycle)
}

// NewOBDInterface creates a diagnostic interface wired to an existing FaultLogger.
// The VIN and initial live data are set here; they get updated as the simulator runs.
func NewOBDInterface(vin string, logger *FaultLogger) *OBDInterface {
	return &OBDInterface{
		vin:         vin,
		faultLogger: logger,
		liveData:    make(map[string]SensorReading),
	}
}

// UpdateLiveData stores the most recent reading for a signal so it's available
// when a scan tool asks for Service 01 (current data).
func (o *OBDInterface) UpdateLiveData(r SensorReading) {
	o.liveData[r.SignalName] = r
}

// CaptureFreezeFrame records a sensor snapshot tied to a specific DTC.
// This gets called by the fault detection logic as soon as a fault is confirmed.
func (o *OBDInterface) CaptureFreezeFrame(dtcCode string) {
	var snapshot []SensorReading
	for _, r := range o.liveData {
		snapshot = append(snapshot, r)
	}
	o.freezeFrames = append(o.freezeFrames, FreezeFrame{
		DTCCode:    dtcCode,
		Readings:   snapshot,
		CapturedAt: time.Now(),
	})
}

// AddPendingDTC registers a fault that's been detected but not yet confirmed
// by a second drive cycle. Real AUTOSAR DEM logic uses a trip-counter for this.
func (o *OBDInterface) AddPendingDTC(code string) {
	for _, existing := range o.pendingDTCs {
		if existing == code {
			return // already pending, don't duplicate
		}
	}
	o.pendingDTCs = append(o.pendingDTCs, code)
}

// HandleRequest routes an incoming OBD request to the correct service handler
// and returns the ECU's response. This is the central dispatch - the same role
// played by the Diagnostic Communication Manager (DCM) in a real AUTOSAR stack.
func (o *OBDInterface) HandleRequest(req OBDRequest) OBDResponse {
	switch req.ServiceID {
	case ServiceShowCurrentData:
		return o.handleService01(req)
	case ServiceShowFreezeFrame:
		return o.handleService02(req)
	case ServiceShowStoredDTCs:
		return o.handleService03(req)
	case ServiceClearDTCs:
		return o.handleService04(req)
	case ServiceShowPendingDTCs:
		return o.handleService07(req)
	case ServiceRequestVehicleInfo:
		return o.handleService09(req)
	default:
		return OBDResponse{
			ServiceID:    req.ServiceID,
			DecodedValue: fmt.Sprintf("Negative response: service 0x%02X not supported", req.ServiceID),
			Timestamp:    time.Now(),
		}
	}
}

// handleService01 returns the current value for a requested PID.
// Encoding follows SAE J1979 Table A-6: each PID has a defined byte layout.
func (o *OBDInterface) handleService01(req OBDRequest) OBDResponse {
	resp := OBDResponse{ServiceID: req.ServiceID, PID: req.PID, Timestamp: time.Now()}

	switch req.PID {
	case PID_SUPPORTED_PIDS:
		// Bitmask indicating which PIDs 01-20 we support.
		// Bit 31 = PID 01, bit 30 = PID 02, etc. We support 04, 05, 0C, 0D, 11.
		supported := uint32(0)
		supported |= (1 << (32 - 0x04)) // engine load
		supported |= (1 << (32 - 0x05)) // coolant temp
		supported |= (1 << (32 - 0x0C)) // RPM
		supported |= (1 << (32 - 0x0D)) // vehicle speed
		supported |= (1 << (32 - 0x11)) // throttle
		resp.RawBytes = []byte{
			byte(supported >> 24),
			byte(supported >> 16),
			byte(supported >> 8),
			byte(supported),
		}
		resp.DecodedValue = fmt.Sprintf("Supported PIDs bitmask: 0x%08X", supported)
		resp.Unit = "-"

	case PID_ENGINE_RPM:
		r, ok := o.liveData["RPM"]
		if !ok {
			resp.DecodedValue = "No data"
			return resp
		}
		// J1979 encoding: (A*256 + B) / 4 = RPM
		raw := uint16(r.Value * 4)
		resp.RawBytes = []byte{byte(raw >> 8), byte(raw)}
		resp.DecodedValue = fmt.Sprintf("%.2f", r.Value)
		resp.Unit = "rpm"

	case PID_COOLANT_TEMP:
		r, ok := o.liveData["COOLANT_TEMP"]
		if !ok {
			resp.DecodedValue = "No data"
			return resp
		}
		// J1979 encoding: A - 40 = temp in C (so A = temp + 40)
		raw := byte(r.Value + 40)
		resp.RawBytes = []byte{raw}
		resp.DecodedValue = fmt.Sprintf("%.1f", r.Value)
		resp.Unit = "C"

	case PID_VEHICLE_SPEED:
		r, ok := o.liveData["VEHICLE_SPEED"]
		if !ok {
			resp.DecodedValue = "No data"
			return resp
		}
		// J1979 encoding: A = speed in km/h, one byte, 0-255
		resp.RawBytes = []byte{byte(r.Value)}
		resp.DecodedValue = fmt.Sprintf("%.1f", r.Value)
		resp.Unit = "km/h"

	case PID_THROTTLE_POSITION:
		r, ok := o.liveData["THROTTLE_POS"]
		if !ok {
			resp.DecodedValue = "No data"
			return resp
		}
		// J1979 encoding: A / 2.55 = throttle %, so A = throttle * 2.55
		raw := byte(r.Value * 2.55)
		resp.RawBytes = []byte{raw}
		resp.DecodedValue = fmt.Sprintf("%.1f", r.Value)
		resp.Unit = "%"

	case PID_BATTERY_VOLTAGE:
		r, ok := o.liveData["BATTERY_VOLT"]
		if !ok {
			resp.DecodedValue = "No data"
			return resp
		}
		// J1979 encoding: (A*256 + B) / 1000 = voltage
		raw := uint16(r.Value * 1000)
		resp.RawBytes = []byte{byte(raw >> 8), byte(raw)}
		resp.DecodedValue = fmt.Sprintf("%.3f", r.Value)
		resp.Unit = "V"

	case PID_OIL_PRESSURE:
		r, ok := o.liveData["OIL_PRESSURE"]
		if !ok {
			resp.DecodedValue = "No data"
			return resp
		}
		// Manufacturer-defined (non-standard PID range), we use 10 * bar as one byte
		raw := byte(r.Value * 10)
		resp.RawBytes = []byte{raw}
		resp.DecodedValue = fmt.Sprintf("%.2f", r.Value)
		resp.Unit = "bar"

	case PID_ENGINE_LOAD:
		// Calculated engine load: we approximate from RPM and throttle
		rpm, hasRPM := o.liveData["RPM"]
		thr, hasThr := o.liveData["THROTTLE_POS"]
		if !hasRPM || !hasThr {
			resp.DecodedValue = "No data"
			return resp
		}
		load := (rpm.Value / 7000.0) * thr.Value // rough approximation
		if load > 100 {
			load = 100
		}
		raw := byte(load * 2.55)
		resp.RawBytes = []byte{raw}
		resp.DecodedValue = fmt.Sprintf("%.1f", load)
		resp.Unit = "%"

	default:
		resp.DecodedValue = fmt.Sprintf("PID 0x%02X not supported", req.PID)
	}

	return resp
}

// handleService02 returns the freeze frame data for a given PID.
// We return the most recently captured freeze frame.
func (o *OBDInterface) handleService02(req OBDRequest) OBDResponse {
	resp := OBDResponse{ServiceID: req.ServiceID, PID: req.PID, Timestamp: time.Now()}

	if len(o.freezeFrames) == 0 {
		resp.DecodedValue = "No freeze frame stored"
		return resp
	}

	// Return the first (oldest) freeze frame - this is what real ECUs do;
	// the oldest fault gets priority in the freeze frame slot.
	ff := o.freezeFrames[0]
	var lines []string
	lines = append(lines, fmt.Sprintf("DTC: %s | Captured: %s", ff.DTCCode, ff.CapturedAt.Format("15:04:05.000")))
	for _, r := range ff.Readings {
		lines = append(lines, fmt.Sprintf("  %-15s = %.2f %s", r.SignalName, r.Value, r.Unit))
	}
	resp.DecodedValue = strings.Join(lines, "\n")
	return resp
}

// handleService03 returns all confirmed stored DTCs from the fault logger.
func (o *OBDInterface) handleService03(req OBDRequest) OBDResponse {
	resp := OBDResponse{ServiceID: req.ServiceID, Timestamp: time.Now()}

	codes := o.storedDTCCodes()
	if len(codes) == 0 {
		resp.DecodedValue = "No stored DTCs"
		return resp
	}

	resp.DecodedValue = fmt.Sprintf("%d stored DTC(s): %s", len(codes), strings.Join(codes, ", "))
	resp.RawBytes = encodeDTCs(codes)
	return resp
}

// handleService04 clears all stored DTCs and freeze frames.
// In a real ECU this also resets the readiness monitors.
func (o *OBDInterface) handleService04(req OBDRequest) OBDResponse {
	cleared := len(o.storedDTCCodes())
	o.faultLogger.log = nil
	o.freezeFrames = nil
	o.pendingDTCs = nil
	return OBDResponse{
		ServiceID:    req.ServiceID,
		DecodedValue: fmt.Sprintf("Cleared %d DTC(s) and all freeze frames", cleared),
		Timestamp:    time.Now(),
	}
}

// handleService07 returns any pending (not yet confirmed) DTCs.
func (o *OBDInterface) handleService07(req OBDRequest) OBDResponse {
	resp := OBDResponse{ServiceID: req.ServiceID, Timestamp: time.Now()}
	if len(o.pendingDTCs) == 0 {
		resp.DecodedValue = "No pending DTCs"
		return resp
	}
	resp.DecodedValue = fmt.Sprintf("%d pending DTC(s): %s",
		len(o.pendingDTCs), strings.Join(o.pendingDTCs, ", "))
	return resp
}

// handleService09 returns vehicle information. PID 0x02 is the VIN.
func (o *OBDInterface) handleService09(req OBDRequest) OBDResponse {
	resp := OBDResponse{ServiceID: req.ServiceID, PID: req.PID, Timestamp: time.Now()}
	switch req.PID {
	case PID_VIN:
		resp.RawBytes = []byte(o.vin)
		resp.DecodedValue = o.vin
		resp.Unit = "-"
	default:
		resp.DecodedValue = fmt.Sprintf("Info PID 0x%02X not supported", req.PID)
	}
	return resp
}

// storedDTCCodes extracts unique DTC codes from the fault logger's log.
func (o *OBDInterface) storedDTCCodes() []string {
	seen := make(map[string]bool)
	var codes []string
	for _, entry := range o.faultLogger.log {
		if !seen[entry.Code] {
			seen[entry.Code] = true
			codes = append(codes, entry.Code)
		}
	}
	return codes
}

// encodeDTCs packs DTC codes into the two-byte-per-code OBD-II wire format.
// Each code like "P0217" becomes 0x02, 0x17 where the high nibble of the
// first byte encodes the system category (P=0, C=4, B=8, U=C).
func encodeDTCs(codes []string) []byte {
	var out []byte
	for _, code := range codes {
		if len(code) != 5 {
			continue
		}
		var category byte
		switch code[0] {
		case 'P':
			category = 0x00
		case 'C':
			category = 0x40
		case 'B':
			category = 0x80
		case 'U':
			category = 0xC0
		}
		// second char is the sub-type digit, third char is second sub-type digit
		// e.g. P0217 -> category=0x00, subtype='0'->0, rest='217'->0x02, 0x17
		var subType byte
		if code[1] >= '0' && code[1] <= '3' {
			subType = (code[1] - '0') << 4
		}
		// remaining three digits as two BCD nibbles
		hi := (code[2] - '0') & 0x0F
		lo := ((code[3]-'0')<<4 | (code[4] - '0')) & 0xFF
		out = append(out, category|subType|hi, lo)
	}
	return out
}

// demonstrateOBDInterface runs through the core OBD-II services to show how
// a scan tool would interact with the ECU. We pre-populate live data and
// trigger a few faults so there's something interesting to read back out.
func demonstrateOBDInterface() {
	fmt.Println("\n========================================")
	fmt.Println("  OBD-II Diagnostic Interface")
	fmt.Println("========================================")

	// Build a fresh fault logger and run some bad readings through it
	// so we have stored DTCs and freeze frames to display.
	logger := NewFaultLogger()
	obd := NewOBDInterface("1HGBH41JXMN109186", logger)

	// Populate live data with a realistic mid-drive snapshot
	liveReadings := []SensorReading{
		{SignalName: "RPM",           Value: 3450,  Unit: "rpm"},
		{SignalName: "COOLANT_TEMP",  Value: 92.5,  Unit: "C"},
		{SignalName: "BATTERY_VOLT",  Value: 13.75, Unit: "V"},
		{SignalName: "VEHICLE_SPEED", Value: 87.0,  Unit: "km/h"},
		{SignalName: "THROTTLE_POS",  Value: 38.0,  Unit: "%"},
		{SignalName: "OIL_PRESSURE",  Value: 2.8,   Unit: "bar"},
	}
	for _, r := range liveReadings {
		obd.UpdateLiveData(r)
	}

	// Simulate two fault events that happened earlier in the drive.
	// Each one logs a DTC and captures a freeze frame at the time of fault.
	faultReadings := []SensorReading{
		{SignalName: "COOLANT_TEMP", Value: 118.5, Unit: "C"},  // overtemp
		{SignalName: "BATTERY_VOLT", Value: 10.8,  Unit: "V"},  // low voltage
		{SignalName: "RPM",          Value: 7900,  Unit: "rpm"}, // overspeed
	}
	for _, r := range faultReadings {
		logger.Validate(r.SignalName, r.Value, r.Unit)
	}
	// Capture a freeze frame tied to the first DTC we logged
	if len(logger.log) > 0 {
		obd.CaptureFreezeFrame(logger.log[0].Code)
	}
	// Add a pending DTC that a previous drive cycle detected but didn't confirm
	obd.AddPendingDTC("P0300") // random misfire detected

	fmt.Println("\n  --- Scan Tool Session ---")
	fmt.Println()

	// Service 09 PID 02: read the VIN first, same as a real scanner does on connect
	printOBDExchange(obd, OBDRequest{ServiceID: ServiceRequestVehicleInfo, PID: PID_VIN})

	// Service 03: read stored DTCs
	printOBDExchange(obd, OBDRequest{ServiceID: ServiceShowStoredDTCs})

	// Service 07: read pending DTCs
	printOBDExchange(obd, OBDRequest{ServiceID: ServiceShowPendingDTCs})

	// Service 02: read freeze frame
	printOBDExchange(obd, OBDRequest{ServiceID: ServiceShowFreezeFrame, PID: PID_ENGINE_RPM})

	// Service 01: query several live PIDs the way a scanner's live data screen would
	fmt.Println("  [Service 01] Live Data PIDs:")
	livePIDs := []struct {
		pid  byte
		name string
	}{
		{PID_ENGINE_RPM, "Engine RPM"},
		{PID_COOLANT_TEMP, "Coolant Temp"},
		{PID_VEHICLE_SPEED, "Vehicle Speed"},
		{PID_THROTTLE_POSITION, "Throttle Position"},
		{PID_BATTERY_VOLTAGE, "Battery Voltage"},
		{PID_OIL_PRESSURE, "Oil Pressure"},
		{PID_ENGINE_LOAD, "Calc. Engine Load"},
	}
	for _, p := range livePIDs {
		resp := obd.HandleRequest(OBDRequest{ServiceID: ServiceShowCurrentData, PID: p.pid})
		fmt.Printf("    PID 0x%02X  %-20s = %-10s %-6s  raw=%X\n",
			p.pid, p.name, resp.DecodedValue, resp.Unit, resp.RawBytes)
	}

	// Service 04: clear everything and confirm the log is empty
	fmt.Println()
	printOBDExchange(obd, OBDRequest{ServiceID: ServiceClearDTCs})
	printOBDExchange(obd, OBDRequest{ServiceID: ServiceShowStoredDTCs})

	fmt.Printf("\n  Session complete. %d DTC entries in log after clear: %d\n",
		len(logger.log), len(obd.storedDTCCodes()))
}

// printOBDExchange formats a single request/response pair the way a scan tool
// would display it in its communication log.
func printOBDExchange(obd *OBDInterface, req OBDRequest) {
	req.Timestamp = time.Now()
	resp := obd.HandleRequest(req)

	serviceNames := map[byte]string{
		ServiceShowCurrentData:    "Service 01 - Current Data",
		ServiceShowFreezeFrame:    "Service 02 - Freeze Frame",
		ServiceShowStoredDTCs:     "Service 03 - Stored DTCs",
		ServiceClearDTCs:          "Service 04 - Clear DTCs",
		ServiceShowPendingDTCs:    "Service 07 - Pending DTCs",
		ServiceRequestVehicleInfo: "Service 09 - Vehicle Info",
	}

	name, ok := serviceNames[req.ServiceID]
	if !ok {
		name = fmt.Sprintf("Service 0x%02X", req.ServiceID)
	}
	if req.PID != 0 {
		name += fmt.Sprintf(" PID=0x%02X", req.PID)
	}

	fmt.Printf("  [%s]\n    -> %s\n\n", name, resp.DecodedValue)
}
