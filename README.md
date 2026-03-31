# Golang Mock ECU

A mock Electronic Control Unit (ECU) built in Go, simulating core concepts from the AUTOSAR automotive software architecture. The project covers sensor signal processing, CAN bus communication, concurrent task scheduling, and fault detection. Essentially the kind of software that runs inside a real car's computer, but written in Go and running on a laptop.

Built for ITSC 4102 at UNC Charlotte.

## Team

Aditya Singh, Hunter Wells, Jubin Kim, Kasper Dziedzic, Nick Goff

## What it does

The simulator is split into four programs that build on each other. Each one introduces new Go concepts while staying grounded in how real automotive software actually works.

**Program 1** covers the basics of how an ECU handles raw signal data. It parses RPM values from strings, rounds voltage readings, cleans up messy signal names, and evaluates fault flags like DTC status and limp mode. Simple operations, but exactly the kind of thing embedded software runs through constantly.

**Program 2** models CAN bus signal processing using arrays, slices, maps, and structs. It includes a fault detection loop that checks live sensor readings like coolant temp, battery voltage, and oil pressure against configurable thresholds and flags anything out of range.

**Program 3** simulates real-time sensor streaming using goroutines and channels. Each sensor runs as its own independent task and pushes readings to a shared channel, which mirrors how parallel runnable tasks work in an AUTOSAR Runtime Environment. There is also a panic/recover example showing how the ECU recovers from bad input without taking down the whole process.

**Program 4** is the CAN layer. Each sensor becomes a proper CAN transmitter node with its own CAN ID (0x100 for RPM, 0x101 for coolant temp, and so on). Values get packed into 8-byte CAN frames using IEEE 754 encoding and broadcast over a virtual bus implemented with a buffered Go channel. A receiver goroutine on the other end decodes each frame back into a SensorReading, the same struct the rest of the simulator uses. Frame latency is tracked per message and typically lands between 45 and 120 microseconds.

## How it maps to AUTOSAR

This is not a real AUTOSAR stack, but the structure is intentionally similar to one.

| AUTOSAR Concept | What we do in Go |
|---|---|
| Software Components (SWCs) | Individual goroutines per sensor signal |
| Runtime Environment (RTE) | Go channels passing data between components |
| CAN Interface layer | can_layer.go handles frame encoding and the virtual bus |
| Diagnostic Event Manager (DEM) | Fault threshold checks in Program 2 |
| Runnable tasks | Goroutines with ticker-based cycle timing |

## Stack

Go 1.24, developed on WSL2 running on Windows. Key packages used are `encoding/binary`, `math`, `sync`, and `time`. No external dependencies.

## Running it

```bash
git clone https://github.com/asingh38-oss/Golang-Mock-ECU.git
cd Golang-Mock-ECU
go mod tidy
go run .
```

## Sample output

```
╔══════════════════════════════════════════╗
║     AUTOSAR Mock ECU Simulator - Go      ║
╚══════════════════════════════════════════╝

[Goroutines] Starting sensor streams...

[Channel] Incoming readings:
  [21:50:45.742] COOLANT_TEMP    = 103.08 C
  [21:50:45.742] BATTERY_VOLT    =  13.90 V
  [21:50:45.891] RPM             = 5185.78 rpm

[Panic/Recover] Testing ECU fault recovery:
  PANIC RECOVERED: runtime error: integer divide by zero - entering safe mode
  ECU load calc: 1000 / 250 = 4

========================================
  CAN Layer: Virtual Bus Simulation
========================================

  [TX] ID=0x100 | RPM             = 5686.93 rpm   | DLC=8 | Bytes=40B636EE147AE148
  [RX] ID=0x100 | RPM             = 5686.93 rpm   | Latency=69µs
  [TX] ID=0x101 | COOLANT_TEMP    =   85.68 C     | DLC=8 | Bytes=40556B851EB851EC
  [RX] ID=0x101 | COOLANT_TEMP    =   85.68 C     | Latency=65µs

  [RX] Done. Received 12 frames total.
```

## Project structure

```
Golang-Mock-ECU/
├── ecu_sim.go      # Programs 1 through 3: data types, structures, concurrency
├── can_layer.go    # Program 4: CAN bus simulation and frame encoding
├── go.mod
└── README.md
```