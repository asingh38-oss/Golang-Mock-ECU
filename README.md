# Golang Mock ECU

A mock Electronic Control Unit built in Go that simulates how real automotive ECU software works. We built this for ITCS 4102 at UNC Charlotte as a way to explore Go while modeling core AUTOSAR concepts like CAN bus communication, sensor processing, fault detection, and drive mode logic.

## Team

Aditya Singh, Hunter Wells, Jubin Kim, Kasper Dziedzic, Nick Goff

## What it does

The project is split across eight files, each handling a different layer of the simulation. They build on each other so reading them in order makes sense.

**ecu_sim.go** is where everything starts. It covers the three core programs for the course: data types and signal processing, data structures and fault detection, and concurrent sensor streaming with goroutine based error recovery.

**can_layer.go** adds a virtual CAN bus. Each sensor gets its own transmitter goroutine with a CAN ID, encodes its value into an 8 byte IEEE 754 frame, and sends it over a buffered channel. A receiver on the other end decodes each frame and measures latency.

**drive_mode.go** is a state machine that switches between ECO, NORMAL, SPORT, and LIMP based on live sensor values. LIMP always takes priority if something critical is wrong.

**scheduler.go** runs each signal on its own timer to match real CAN matrix timing. RPM and speed fire every 20ms, throttle every 50ms, and coolant and battery every 100ms.

**fault_logger.go and obd_interface.go** validate sensor readings against known ranges and log Diagnostic Trouble Codes when something goes out of bounds. The OBD interface simulates a scan tool with service modes 01 through 09.

**sensor_model.go** replaces random values with a physics simulation where RPM, coolant temp, battery voltage, oil pressure, and vehicle speed all respond to throttle and gear input.

**ecuserver.go** spins up an HTTP server on port 8080 so you can query live sensor data and fault codes from a browser.

## How it maps to AUTOSAR

| AUTOSAR Concept | What we built |
|---|---|
| Software Components | Goroutines per sensor signal |
| Runtime Environment | Go channels between components |
| CAN Interface | can_layer.go frame encoding and virtual bus |
| Diagnostic Event Manager | fault_logger.go DTC generation |
| Periodic Runnable Tasks | scheduler.go ticker based goroutines |
| State Management | drive_mode.go FSM |
| Diagnostic Communication Manager | obd_interface.go service modes |

## Stack

Go 1.24 on WSL2 running on Windows. No external dependencies, everything uses the standard library.

## Running it

```bash
git clone https://github.com/asingh38-oss/Golang-Mock-ECU.git
cd Golang-Mock-ECU
go mod tidy
go run .
```

The HTTP server starts at the end and blocks on port 8080. Press Ctrl+C to stop. While it is running you can hit these in a browser:

```
http://localhost:8080/sensors
http://localhost:8080/sensors/RPM
http://localhost:8080/faults
```

## Project structure

```
Golang-Mock-ECU/
├── ecu_sim.go        # programs 1 through 3: data types, structures, concurrency
├── can_layer.go      # CAN bus simulation and frame encoding
├── drive_mode.go     # drive mode state machine
├── scheduler.go      # periodic message scheduler
├── fault_logger.go   # signal validation and DTC logging
├── obd_interface.go  # OBD-II scan tool simulation
├── sensor_model.go   # physics coupled sensor model
├── ecuserver.go      # ECU HTTP server
├── go.mod
└── README.md
```