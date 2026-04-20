package main
 
import (
	"fmt"
	"math"
	"strings"
	"time"
)
 

var GearRatio = map[int]float64{
	0: 0.0,
	1: 13.5,
	2: 7.9,
	3: 5.1,
	4: 3.7,
	5: 2.9,
	6: 2.3,
}
 

const WheelCircumference = 1.931
 

type EngineState struct {
	RPM          float64 
	ThrottlePos  float64 
	CoolantTemp  float64 
	BatteryVolt  float64 
	OilPressure  float64 
	VehicleSpeed float64 
	Gear         int     
	AmbientTemp  float64 
}
 

type PhysicsConfig struct {
	
	ThermalRiseRate  float64 
	ThermalDecayRate float64 
	ThermostatTemp   float64 
 
	
	NominalVoltage   float64 
	VoltageDropRate  float64 
	AlternatorGain   float64 
 
	
	OilPressureBase  float64 
	OilPressureGain  float64 
 
	
	RPMResponseRate  float64 
	IdleRPM          float64 
	MaxRPM           float64 
}
 

func defaultPhysicsConfig() PhysicsConfig {
	return PhysicsConfig{
		ThermalRiseRate:  8.0,
		ThermalDecayRate: 3.0,
		ThermostatTemp:   82.0,
		NominalVoltage:   14.2,
		VoltageDropRate:  0.04,
		AlternatorGain:   0.0003,
		OilPressureBase:  1.2,
		OilPressureGain:  0.55,
		RPMResponseRate:  1800.0,
		IdleRPM:          820.0,
		MaxRPM:           6800.0,
	}
}
 

type SensorModel struct {
	State  EngineState
	Config PhysicsConfig
}
 

func NewSensorModel(cfg PhysicsConfig, ambientTemp float64) *SensorModel {
	return &SensorModel{
		Config: cfg,
		State: EngineState{
			RPM:          cfg.IdleRPM,
			ThrottlePos:  0.0,
			CoolantTemp:  ambientTemp + 5.0, 
			BatteryVolt:  cfg.NominalVoltage,
			OilPressure:  cfg.OilPressureBase,
			VehicleSpeed: 0.0,
			Gear:         1,
			AmbientTemp:  ambientTemp,
		},
	}
}
 

func (m *SensorModel) Step(throttleDemand float64, gear int, dt float64) {
	cfg := m.Config
	s := &m.State
 
	
	throttleDemand = math.Max(0, math.Min(100, throttleDemand))
	if gear < 0 || gear > 6 {
		gear = 1
	}
	s.ThrottlePos = throttleDemand
	s.Gear = gear
 
	
	targetRPM := cfg.IdleRPM + (throttleDemand/100.0)*(cfg.MaxRPM-cfg.IdleRPM)
	rpmDelta := (targetRPM - s.RPM) * math.Min(1.0, cfg.RPMResponseRate*dt/cfg.MaxRPM)
	s.RPM = math.Max(cfg.IdleRPM, math.Min(cfg.MaxRPM, s.RPM+rpmDelta))
 
	
	rpmLoadFraction := (s.RPM - cfg.IdleRPM) / (cfg.MaxRPM - cfg.IdleRPM)
	heatInput := cfg.ThermalRiseRate * rpmLoadFraction * dt
 
	var heatLoss float64
	if s.CoolantTemp > cfg.ThermostatTemp {
		
		overTemp := s.CoolantTemp - cfg.ThermostatTemp
		heatLoss = cfg.ThermalDecayRate * (1.0 + overTemp/20.0) * dt
	}
	s.CoolantTemp += heatInput - heatLoss
	s.CoolantTemp = math.Max(s.AmbientTemp, s.CoolantTemp)
 
	
	electricalLoad := (throttleDemand / 100.0) * (s.RPM / cfg.MaxRPM)
	voltageSag := cfg.VoltageDropRate * electricalLoad
	alternatorBoost := cfg.AlternatorGain * s.RPM
	s.BatteryVolt = cfg.NominalVoltage - voltageSag + alternatorBoost
	s.BatteryVolt = math.Max(11.0, math.Min(15.5, s.BatteryVolt))
 
	
	rpmAboveIdle := math.Max(0, s.RPM-cfg.IdleRPM)
	s.OilPressure = cfg.OilPressureBase + cfg.OilPressureGain*(rpmAboveIdle/1000.0)
	s.OilPressure = math.Max(0.8, math.Min(6.0, s.OilPressure))
 
	
	ratio := GearRatio[gear]
	if ratio > 0 {
		s.VehicleSpeed = (s.RPM / ratio) * WheelCircumference * 60.0 / 1000.0
	} else {
		s.VehicleSpeed = 0.0 
	}
	s.VehicleSpeed = math.Max(0, math.Min(250, s.VehicleSpeed))
}
 

func (m *SensorModel) Readings() []SensorReading {
	s := m.State
	now := time.Now()
	return []SensorReading{
		{SignalName: "RPM",           Value: math.Round(s.RPM*10) / 10,          Unit: "rpm",  Timestamp: now},
		{SignalName: "COOLANT_TEMP",  Value: math.Round(s.CoolantTemp*10) / 10,  Unit: "C",    Timestamp: now},
		{SignalName: "BATTERY_VOLT",  Value: math.Round(s.BatteryVolt*100) / 100, Unit: "V",   Timestamp: now},
		{SignalName: "OIL_PRESSURE",  Value: math.Round(s.OilPressure*100) / 100, Unit: "bar", Timestamp: now},
		{SignalName: "VEHICLE_SPEED", Value: math.Round(s.VehicleSpeed*10) / 10, Unit: "km/h", Timestamp: now},
		{SignalName: "THROTTLE_POS",  Value: math.Round(s.ThrottlePos*10) / 10,  Unit: "%",    Timestamp: now},
	}
}
 

type DriveScenario struct {
	Label    string
	Throttle float64
	Gear     int
	Duration float64 
}
 

func demonstrateSensorModel() {
	fmt.Println("\n========================================")
	fmt.Println("  Physics-Coupled Sensor Model")
	fmt.Println("========================================")
 
	cfg := defaultPhysicsConfig()
	model := NewSensorModel(cfg, 20.0) 
 
	faultLogger := NewFaultLogger()
	driveCtrl := NewDriveModeController(defaultDriveModeConfig())
 
	
	scenarios := []DriveScenario{
		{"Cold start / idle",         0,   1, 8.0},
		{"Gentle city pull",         25,   2, 5.0},
		{"City cruise",              30,   3, 6.0},
		{"On-ramp acceleration",     80,   3, 4.0},
		{"Highway cruise",           35,   5, 8.0},
		{"Hard overtake",            95,   4, 3.0},
		{"Back to cruise",           30,   5, 6.0},
		{"Decelerate / coast",        5,   3, 4.0},
		{"Stop / idle",               0,   1, 5.0},
	}
 
	
	const dt = 0.5
 
	fmt.Printf("\n  Ambient temp: %.0fC  |  Idle RPM: %.0f  |  Rev limit: %.0f\n",
		20.0, cfg.IdleRPM, cfg.MaxRPM)
	fmt.Println()
 
	
	header := fmt.Sprintf("  %-26s %-6s %-5s  %-7s  %-7s  %-7s  %-6s  %-6s  %-7s  %-8s",
		"Scenario", "t(s)", "Thr%", "RPM", "Speed", "Coolant", "Batt V", "Oil", "Mode", "Faults")
	fmt.Println(header)
	fmt.Println("  " + strings.Repeat("-", len(header)-2))
 
	var simTime float64
	totalFaults := 0
 
	for _, scenario := range scenarios {
		steps := int(scenario.Duration / dt)
		for step := 0; step < steps; step++ {
			model.Step(scenario.Throttle, scenario.Gear, dt)
			simTime += dt
 
			readings := model.Readings()
			s := model.State
 
			
			stepFaults := 0
			for _, r := range readings {
				if !faultLogger.Validate(r.SignalName, r.Value, r.Unit) {
					stepFaults++
					totalFaults++
				}
			}
 
			
			mode := driveCtrl.Evaluate(s.RPM, s.CoolantTemp, s.BatteryVolt, s.ThrottlePos)
 
			
			if step%4 == 0 || step == steps-1 {
				faultMark := ""
				if stepFaults > 0 {
					faultMark = fmt.Sprintf("  !! %d", stepFaults)
				}
				fmt.Printf("  %-26s %5.1f  %4.0f   %6.0f  %6.1f   %6.1f    %5.2f  %4.2f   %-8s%s\n",
					scenario.Label,
					simTime,
					s.ThrottlePos,
					s.RPM,
					s.VehicleSpeed,
					s.CoolantTemp,
					s.BatteryVolt,
					s.OilPressure,
					mode,
					faultMark,
				)
			}
		}
	}
 
	
	fmt.Printf("\n  Simulation complete. Simulated %.0fs of driving.\n", simTime)
	fmt.Printf("  Drive mode transitions: %d\n", driveCtrl.transitions)
	fmt.Printf("  Total fault events logged: %d\n", totalFaults)
 
	
	if len(faultLogger.log) > 0 {
		fmt.Println("\n  --- DTCs triggered during drive ---")
		fmt.Println()
		faultLogger.PrintDTCLog()
	} else {
		fmt.Println("\n  No DTCs triggered during this drive cycle.")
	}
 
	fmt.Printf("\n  Final engine state:\n")
	s := model.State
	fmt.Printf("    RPM:          %.0f\n", s.RPM)
	fmt.Printf("    Coolant:      %.1f C\n", s.CoolantTemp)
	fmt.Printf("    Battery:      %.2f V\n", s.BatteryVolt)
	fmt.Printf("    Oil pressure: %.2f bar\n", s.OilPressure)
	fmt.Printf("    Speed:        %.1f km/h\n", s.VehicleSpeed)
	fmt.Printf("    Drive mode:   %s\n", driveCtrl.CurrentMode)
}
