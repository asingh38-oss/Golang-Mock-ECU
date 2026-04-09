package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// ScheduledSignal defines one signal that the scheduler manages.
// Each signal has its own CAN ID, name, value range, and transmission interval.
// In AUTOSAR this configuration lives in the communication matrix (.arxml),
// and the RTE calls each runnable on its configured period automatically.
type ScheduledSignal struct {
	CANID    uint32
	Name     string
	Unit     string
	Min, Max float64
	Interval time.Duration // how often this signal gets sent onto the bus
}

// SchedulerStats tracks what happened during a scheduler run so we can
// verify that each signal fired at roughly the right frequency.
type SchedulerStats struct {
	mu     sync.Mutex
	counts map[string]int // how many times each signal was transmitted
}

func newSchedulerStats() *SchedulerStats {
	return &SchedulerStats{counts: make(map[string]int)}
}

func (s *SchedulerStats) record(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[name]++
}

func (s *SchedulerStats) get(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[name]
}

// scheduledTransmitter is the goroutine that handles one signal.
// It ticks on its own interval and sends a CAN frame every time it fires.
// The done channel lets the main goroutine shut everything down cleanly.
func scheduledTransmitter(sig ScheduledSignal, bus *CANBus, stats *SchedulerStats, done <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(sig.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			// generate a fresh reading each tick
			rawVal := sig.Min + rand.Float64()*(sig.Max-sig.Min)
			value := math.Round(rawVal*100) / 100

			frame := CANFrame{
				ID:        sig.CANID,
				DLC:       8,
				Data:      packFloat64(value),
				Timestamp: t,
			}

			bus.Send(frame)
			stats.record(sig.Name)

			fmt.Printf("  [SCHED TX] %dms  ID=0x%03X  %-15s = %7.2f %-5s\n",
				sig.Interval.Milliseconds(), sig.CANID, sig.Name, value, sig.Unit)
		}
	}
}

// demonstrateScheduler shows how multiple signals can each run on their own
// independent timer, the same way AUTOSAR's RTE schedules periodic tasks.
//
// Signal periods chosen here match typical CAN matrix timing from real vehicles:
//   - RPM and vehicle speed at 20ms (50Hz) - fast signals drivers feel immediately
//   - Throttle position at 50ms (20Hz) - slightly slower, still responsive
//   - Coolant temp and battery at 100ms (10Hz) - slow-changing, no need to flood the bus
func demonstrateScheduler() {
	fmt.Println("\n========================================")
	fmt.Println("  Periodic Message Scheduler")
	fmt.Println("========================================")

	signals := []ScheduledSignal{
		{CAN_ID_RPM,           "RPM",           "rpm",  800,  6500, 20 * time.Millisecond},
		{CAN_ID_VEHICLE_SPEED, "VEHICLE_SPEED", "km/h", 0,    120,  20 * time.Millisecond},
		{0x104,                "THROTTLE_POS",  "%",    0,    100,  50 * time.Millisecond},
		{CAN_ID_COOLANT_TEMP,  "COOLANT_TEMP",  "C",    70,   105,  100 * time.Millisecond},
		{CAN_ID_BATTERY_VOLT,  "BATTERY_VOLT",  "V",    12.0, 14.8, 100 * time.Millisecond},
	}

	fmt.Println("\n  Signal schedule:")
	for _, s := range signals {
		fmt.Printf("    ID=0x%03X  %-15s  every %dms\n", s.CANID, s.Name, s.Interval.Milliseconds())
	}

	// run the scheduler for 200ms - long enough to see the different rates clearly
	runDuration := 200 * time.Millisecond

	bus := NewCANBus(100)
	stats := newSchedulerStats()
	done := make(chan struct{})

	var wg sync.WaitGroup

	fmt.Printf("\n  Running scheduler for %dms...\n\n", runDuration.Milliseconds())

	for _, sig := range signals {
		wg.Add(1)
		go scheduledTransmitter(sig, bus, stats, done, &wg)
	}

	// let it run, then shut everything down
	time.Sleep(runDuration)
	close(done)
	wg.Wait()

	// drain any frames still sitting in the bus buffer
	drainCount := 0
	for len(bus.frames) > 0 {
		<-bus.frames
		drainCount++
	}

	// show the actual transmission counts vs what we expected
	fmt.Println("\n  Transmission summary:")
	fmt.Printf("  %-15s  %8s  %8s\n", "Signal", "Expected", "Actual")
	fmt.Println("  " + repeatChar('-', 35))

	for _, s := range signals {
		// expected = how many intervals fit in the run window
		expected := int(runDuration / s.Interval)
		actual := stats.get(s.Name)
		fmt.Printf("  %-15s  %8d  %8d\n", s.Name, expected, actual)
	}

	fmt.Printf("\n  (Drained %d buffered frames after shutdown)\n", drainCount)
}
