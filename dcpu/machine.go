package dcpu

import (
	"errors"
	"fmt"
	"github.com/kballard/dcpu16/dcpu/core"
	"io"
	"strconv"
	"strings"
	"time"
)

type Machine struct {
	State      core.State
	Video      Video
	Keyboard   Keyboard
	stopper    chan<- struct{}
	stopped    <-chan error
	cycleCount uint
	startTime  time.Time
}

type MachineError struct {
	UnderlyingError error
	PC              core.Word
}

func (err *MachineError) Error() string {
	return fmt.Sprintf("machine error occurred; PC: %#x (%v)", err.PC, err.UnderlyingError)
}

const DefaultClockRate ClockRate = 100000 // 100KHz

// Start boots up the machine, with a clock rate of 1 / period
// 10MHz would be expressed as (Microsecond / 10)
func (m *Machine) Start(rate ClockRate) (err error) {
	if m.stopped != nil {
		return errors.New("Machine has already started")
	}
	if err = m.Video.Init(); err != nil {
		return
	}
	defer func() {
		if err != nil {
			m.Video.Close()
		}
	}()
	if err = m.Video.MapToMachine(0x8000, m); err != nil {
		return
	}
	if err = m.Keyboard.MapToMachine(0x9000, m); err != nil {
		return
	}
	stopper := make(chan struct{}, 1)
	m.stopper = stopper
	stopped := make(chan error, 1)
	m.stopped = stopped
	m.cycleCount = 0
	m.startTime = time.Now()
	go func() {
		// we want an acurate cycle counter
		// Unfortunately, time.NewTicker drops cycles on the floor if it can't keep up
		// So lets instead switch to running as many cycles as we need before using any
		// timed delays
		cycleChan := make(chan time.Time, 1)
		scanrate := time.NewTicker(time.Second / 60) // 60Hz
		var stoperr error
		nextTime := time.Now()
		period := rate.ToDuration()
		cycleChan <- nextTime
		var timerChan <-chan time.Time
		// runCycle needs to be split into a function, because we want to call it if
		// any of two channels has a value
		runCycle := func() bool {
			if err := m.State.StepCycle(); err != nil {
				stoperr = &MachineError{err, m.State.PC()}
				return false
			}
			m.cycleCount++
			m.Keyboard.PollKeys()
			nextTime = nextTime.Add(period)
			now := time.Now()
			if now.Before(nextTime) {
				// delay the cycle
				timerChan = time.After(nextTime.Sub(now))
			} else {
				// trigger a cycle now
				cycleChan <- now
			}
			return true
		}
	loop:
		for {
			select {
			case _ = <-scanrate.C:
				m.Video.UpdateStats(&m.State, m.cycleCount)
				m.Video.Flush()
			case _ = <-timerChan:
				if !runCycle() {
					break loop
				}
			case _ = <-cycleChan:
				if !runCycle() {
					break loop
				}
			case _ = <-stopper:
				break loop
			}
		}
		scanrate.Stop()
		stopped <- stoperr
		close(stopped)
	}()
	return nil
}

// Stop stops the machine. Returns an error if it's already stopped.
// If the machine has halted due to an error, that error is returned.
func (m *Machine) Stop() error {
	if m.stopped == nil {
		return errors.New("Machine has not started")
	}
	m.Video.UnmapFromMachine(0x8000, m)
	m.Keyboard.UnmapFromMachine(0x9000, m)
	m.stopper <- struct{}{}
	m.Video.Close()
	err := <-m.stopped
	close(m.stopper)
	m.stopper = nil
	m.stopped = nil
	return err
}

// ClockRate represents the clock rate of the machine
type ClockRate int64

func (c ClockRate) String() string {
	rate := float64(c)
	// We want to do some rounding instead of pure truncation
	// 99.999KHz shouldn't be showing as 99KHz
	// Lets try rounding to 1 decimal place
	suffix := "Hz"
	if rate >= 1e6 {
		rate /= 1e6
		suffix = "MHz"
	} else if rate >= 1e3 {
		rate /= 1e3
		suffix = "KHz"
	}
	ratestr := strconv.FormatFloat(rate, 'f', 1, 64)
	if strings.HasSuffix(ratestr, ".0") {
		ratestr = ratestr[:len(ratestr)-2]
	}
	return fmt.Sprintf("%s%s", ratestr, suffix)
}

func (c *ClockRate) Set(str string) error {
	var rate int64
	var suffix string
	if n, err := fmt.Sscanf(str, "%d%s", &rate, &suffix); err != nil && !(n == 1 && err == io.EOF) {
		return err
	}
	if rate <= 0 {
		return errors.New("clock rate must be positive")
	}
	switch strings.ToLower(suffix) {
	case "mhz":
		rate *= 1e6
	case "khz":
		rate *= 1e3
	case "hz", "":
	default:
		return errors.New(fmt.Sprintf("unknown suffix %#v", suffix))
	}
	*c = ClockRate(rate)
	return nil
}

// ToDuration converts the ClockRate to a time.Duration that represents
// the period of one clock cycle
func (c ClockRate) ToDuration() time.Duration {
	return time.Second / time.Duration(c)
}

// EffectiveClockRate returns the current observed rate that the machine
// is running at, as an average since the last Start()
func (m *Machine) EffectiveClockRate() ClockRate {
	duration := time.Since(m.startTime)
	cycles := m.cycleCount
	return ClockRate(float64(cycles) / duration.Seconds())
}

// If the machine has already halted due to an error, that error is returned.
// Otherwise, nil is returned.
// If the machine has not started, an error is returned.
func (m *Machine) HasError() error {
	if m.stopped == nil {
		return errors.New("Machine has not started")
	}
	select {
	case err := <-m.stopped:
		m.Video.Close()
		close(m.stopper)
		m.stopper = nil
		m.stopped = nil
		return err
	default:
	}
	return nil
}
