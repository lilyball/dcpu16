package dcpu

import (
	"errors"
	"fmt"
	"github.com/kballard/dcpu16/dcpu/core"
	"io"
	"strings"
	"time"
)

type Machine struct {
	State      core.State
	Video      Video
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
func (m *Machine) Start(rate ClockRate) error {
	if m.stopped != nil {
		return errors.New("Machine has already started")
	}
	if err := m.Video.Init(); err != nil {
		return err
	}
	if err := m.Video.MapToMachine(0x8000, m); err != nil {
		m.Video.Close()
		return err
	}
	stopper := make(chan struct{}, 1)
	m.stopper = stopper
	stopped := make(chan error, 1)
	m.stopped = stopped
	m.cycleCount = 0
	m.startTime = time.Now()
	go func() {
		ticker := time.NewTicker(rate.ToDuration())
		scanrate := time.NewTicker(time.Second / 60) // 60Hz
		var stoperr error
	loop:
		for {
			select {
			case _ = <-scanrate.C:
				m.Video.Flush()
			case _ = <-ticker.C:
				if err := m.State.StepCycle(); err != nil {
					stoperr = &MachineError{err, m.State.PC()}
					break loop
				}
				m.cycleCount++
				m.Video.HandleChanges()
			case _ = <-stopper:
				break loop
			}
		}
		ticker.Stop()
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
	m.Video.Close()
	m.stopper <- struct{}{}
	err := <-m.stopped
	close(m.stopper)
	m.stopper = nil
	m.stopped = nil
	return err
}

// ClockRate represents the clock rate of the machine
type ClockRate int64

func (c ClockRate) String() string {
	rate := int64(c)
	suffix := "Hz"
	if rate >= 1e6 {
		rate /= 1e6
		suffix = "MHz"
	} else if rate >= 1e3 {
		rate /= 1e3
		suffix = "KHz"
	}
	return fmt.Sprintf("%d%s", rate, suffix)
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
