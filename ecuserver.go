package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ECUServer struct {
	logger   *FaultLogger
	readings map[string]SensorReading
}

func NewECUServer(logger *FaultLogger) *ECUServer {
	return &ECUServer{
		logger:   logger,
		readings: make(map[string]SensorReading),
	}
}

func (s *ECUServer) UpdateReading(r SensorReading) {
	s.readings[r.SignalName] = r
}

func (s *ECUServer) handleSensors(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/sensors/")

	if name == "" || name == "sensors" {
		all := make([]SensorReading, 0, len(s.readings))
		for _, v := range s.readings {
			all = append(all, v)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(all)
		return
	}

	reading, ok := s.readings[strings.ToUpper(name)]
	if !ok {
		http.Error(w, "signal not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reading)
}

func (s *ECUServer) handleFaults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.logger.log)
}

func demonstrateECUServer() {
	fmt.Println("\n========================================")
	fmt.Println("  ECU HTTP Server")
	fmt.Println("========================================")

	logger := NewFaultLogger()
	server := NewECUServer(logger)

	readings := []SensorReading{
		{SignalName: "RPM", Value: 3200, Unit: "rpm", Timestamp: time.Now()},
		{SignalName: "COOLANT_TEMP", Value: 92.5, Unit: "C", Timestamp: time.Now()},
		{SignalName: "BATTERY_VOLT", Value: 13.75, Unit: "V", Timestamp: time.Now()},
		{SignalName: "VEHICLE_SPEED", Value: 87.0, Unit: "km/h", Timestamp: time.Now()},
		{SignalName: "OIL_PRESSURE", Value: 0.2, Unit: "bar", Timestamp: time.Now()},
	}

	for _, r := range readings {
		server.UpdateReading(r)
		logger.Validate(r.SignalName, r.Value, r.Unit)
	}

	http.HandleFunc("/sensors/", server.handleSensors)
	http.HandleFunc("/sensors", server.handleSensors)
	http.HandleFunc("/faults", server.handleFaults)

	fmt.Println("\n  Routes:")
	fmt.Println("    GET /sensors          -> all current sensor readings")
	fmt.Println("    GET /sensors/{name}   -> single signal e.g. /sensors/RPM")
	fmt.Println("    GET /faults           -> active DTC fault log")
	fmt.Println("\n  Listening on http://localhost:8080")
	fmt.Println("  Press Ctrl+C to stop.\n")

	http.ListenAndServe(":8080", nil)
}
