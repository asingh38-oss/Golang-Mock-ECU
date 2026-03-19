# Mock ECU — AUTOSAR-Inspired Embedded System Simulator

A mock Electronic Control Unit (ECU) written in Go, simulating core AUTOSAR software architecture concepts including modular runnable tasks, scheduling layers, and inter-component communication patterns found in real automotive software stacks.

## What it does

The simulator runs three programs that mirror how a real ECU processes data:

**Program 1 — Data Types & Built-in Methods**
Demonstrates how an ECU handles raw sensor signals — parsing RPM values, formatting voltage readings, cleaning signal names, and evaluating fault flags like DTC status and limp mode.

**Program 2 — Data Structures & Control Structures**
Models CAN bus signal processing using arrays, slices, maps, and structs. Includes basic fault detection logic that checks live sensor readings (coolant temp, battery voltage, oil pressure) against configurable thresholds.

**Program 3 — Concurrency & Exception Handling**
Simulates real-time sensor streaming using goroutines and channels — each sensor runs independently and pushes readings to a shared channel. Includes panic/recover to simulate ECU fault recovery behavior under bad input.

## Stack

- **Language:** Go 1.24
- **Environment:** WSL2 / Linux
- **Concepts:** AUTOSAR architecture, goroutines, channels, structs, CAN bus simulation, fault detection

## Running it

```bash
git clone https://github.com/asingh38-oss/mock-ecu
cd mock-ecu
go run main.go
```

## Sample output

```
╔══════════════════════════════════════════╗
║     AUTOSAR Mock ECU Simulator - Go      ║
╚══════════════════════════════════════════╝

[Goroutines] Starting sensor streams...

[Channel] Incoming readings:
  [14:03:22.451] RPM             = 4821.33 rpm
  [14:03:22.502] COOLANT_TEMP    = 98.71 C
  [14:03:22.513] BATTERY_VOLT    = 13.45 V
  ...

[Panic/Recover] Testing ECU fault recovery:
  PANIC RECOVERED: runtime error: integer divide by zero - entering safe mode
  ECU load calc: 1000 / 250 = 4
```

## Architecture notes

The design mirrors AUTOSAR's layered model: sensor abstraction, signal processing, and fault management are separated into distinct functions rather than mixed together. Goroutines stand in for the parallel runnable tasks that an AUTOSAR Runtime Environment (RTE) would schedule on real hardware.
