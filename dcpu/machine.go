package dcpu

import (
	"errors"
	"fmt"
	"github.com/kballard/dcpu16/dcpu/core"
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

const DefaultClockRate = time.Microsecond * 10

// Start boots up the machine, with a clock rate of 1 / period
// 10MHz would be expressed as (Microsecond / 10)
func (m *Machine) Start(period time.Duration) error {
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
		ticker := time.NewTicker(period)
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
		m.Video.Close()
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
	m.stopper <- struct{}{}
	err := <-m.stopped
	close(m.stopper)
	m.stopper = nil
	m.stopped = nil
	return err
}

// EffectiveClockRate returns the current observed rate that the machine
// is running at, as an average since the last Start()
func (m *Machine) EffectiveClockRate() uint {
	duration := time.Since(m.startTime)
	cycles := m.cycleCount
	return uint(float64(cycles) / duration.Seconds())
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
		close(m.stopper)
		m.stopper = nil
		m.stopped = nil
		return err
	default:
	}
	return nil
}
