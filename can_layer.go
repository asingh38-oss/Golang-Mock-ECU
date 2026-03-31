package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// CAN IDs for each sensor signal. In real AUTOSAR these come from the .arxml config file,
// but we're hardcoding them here to keep things simple.
// Lower ID means higher priority on the bus, that's standard CAN arbitration.
const (
	CAN_ID_RPM           = 0x100
	CAN_ID_COOLANT_TEMP  = 0x101
	CAN_ID_BATTERY_VOLT  = 0x102
	CAN_ID_VEHICLE_SPEED = 0x103
)

// lookup tables so the receiver knows what it's looking at
var canIDToName = map[uint32]string{
	CAN_ID_RPM:           "RPM",
	CAN_ID_COOLANT_TEMP:  "COOLANT_TEMP",
	CAN_ID_BATTERY_VOLT:  "BATTERY_VOLT",
	CAN_ID_VEHICLE_SPEED: "VEHICLE_SPEED",
}

var canIDToUnit = map[uint32]string{
	CAN_ID_RPM:           "rpm",
	CAN_ID_COOLANT_TEMP:  "C",
	CAN_ID_BATTERY_VOLT:  "V",
	CAN_ID_VEHICLE_SPEED: "km/h",
}

// CANFrame represents a single message on the CAN bus.
// In a real car this is a physical electrical signal on a two-wire bus,
// but here it's just a struct we pass around through channels.
type CANFrame struct {
	ID        uint32    // identifies who sent this and what signal it carries
	DLC       uint8     // data length code, how many bytes are in Data (max 8)
	Data      [8]byte   // the actual payload, sensor value packed into raw bytes
	Timestamp time.Time // when the frame was sent so we can measure latency
}

// CANBus is our virtual bus. Any goroutine can drop a frame on it with Send
// and any goroutine can pull one off with Receive, same idea as a real shared wire.
type CANBus struct {
	frames chan CANFrame
}

func NewCANBus(bufferSize int) *CANBus {
	return &CANBus{
		frames: make(chan CANFrame, bufferSize),
	}
}

func (b *CANBus) Send(frame CANFrame) {
	b.frames <- frame
}

func (b *CANBus) Receive() (CANFrame, bool) {
	frame, ok := <-b.frames
	return frame, ok
}

// packFloat64 takes a sensor reading and turns it into 8 raw bytes.
// Real ECUs do this with signal scaling and bit packing (defined in .dbc files),
// but using the full IEEE 754 float keeps things readable for now.
func packFloat64(val float64) [8]byte {
	var buf [8]byte
	bits := math.Float64bits(val)
	binary.BigEndian.PutUint64(buf[:], bits)
	return buf
}

// unpackFloat64 reverses the above - raw bytes back to a float
func unpackFloat64(data [8]byte) float64 {
	bits := binary.BigEndian.Uint64(data[:])
	return math.Float64frombits(bits)
}

// CANTransmitter simulates an ECU node broadcasting a sensor signal onto the bus.
// Each sensor gets its own goroutine, which is roughly how AUTOSAR runnable tasks work.
// They each run on their own schedule and don't block each other.
func CANTransmitter(canID uint32, name, unit string, min, max float64, bus *CANBus, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < 3; i++ {
		// generate a random sensor value in the valid range
		rawValue := min + rand.Float64()*(max-min)
		value := math.Round(rawValue*100) / 100

		frame := CANFrame{
			ID:        canID,
			DLC:       8,
			Data:      packFloat64(value),
			Timestamp: time.Now(),
		}

		bus.Send(frame)

		fmt.Printf("  [TX] ID=0x%03X | %-15s = %7.2f %-5s | DLC=%d | Bytes=%X\n",
			frame.ID, name, value, unit, frame.DLC, frame.Data)

		time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)+50))
	}
}

// CANReceiver acts like a second ECU on the bus - a dashboard or body control module
// that's listening for frames and decoding them back into usable sensor readings.
// It knows what to expect because we told it how many frames total will be sent.
func CANReceiver(bus *CANBus, totalFrames int) {
	received := 0
	fmt.Println("\n  [RX] Receiver online, waiting for frames...")

	for received < totalFrames {
		frame, ok := bus.Receive()
		if !ok {
			break
		}

		value := unpackFloat64(frame.Data)
		name := canIDToName[frame.ID]
		unit := canIDToUnit[frame.ID]

		// reconstruct the SensorReading so it plugs back into the rest of the simulator
		reading := SensorReading{
			SignalName: name,
			Value:      value,
			Unit:       unit,
			Timestamp:  frame.Timestamp,
		}

		fmt.Printf("  [RX] ID=0x%03X | %-15s = %7.2f %-5s | Latency=%s\n",
			frame.ID,
			reading.SignalName,
			reading.Value,
			reading.Unit,
			time.Since(frame.Timestamp).Round(time.Microsecond),
		)

		received++
	}

	fmt.Printf("\n  [RX] Done. Received %d frames total.\n", received)
}

func demonstrateCANLayer() {
	fmt.Println("\n========================================")
	fmt.Println("  CAN Layer: Virtual Bus Simulation")
	fmt.Println("========================================")

	bus := NewCANBus(50)

	nodes := []struct {
		canID    uint32
		name     string
		unit     string
		min, max float64
	}{
		{CAN_ID_RPM, "RPM", "rpm", 800, 6500},
		{CAN_ID_COOLANT_TEMP, "COOLANT_TEMP", "C", 70, 105},
		{CAN_ID_BATTERY_VOLT, "BATTERY_VOLT", "V", 12.0, 14.8},
		{CAN_ID_VEHICLE_SPEED, "VEHICLE_SPEED", "km/h", 0, 120},
	}

	// 4 sensors x 3 readings each
	totalFrames := len(nodes) * 3

	// receiver runs in the background while transmitters do their thing
	go CANReceiver(bus, totalFrames)

	// small delay so the receiver is ready before frames start coming in
	time.Sleep(10 * time.Millisecond)

	fmt.Println("\n  [TX] Transmitters online:")
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go CANTransmitter(node.canID, node.name, node.unit, node.min, node.max, bus, &wg)
	}

	// wait for all transmitters to finish, give receiver a moment to drain
	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}