package alarm

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// State is Armed, Disarmed or Triggered.
type State int

const (
	// Disarmed is the initial state.
	Disarmed = iota
	// Arming the alarm.
	Arming
	// Armed is when the alarm is ready.
	Armed
	// Triggering is when the alarm has been triggered, the user has a few seconds to disarm the alarm.
	Triggering
	// Triggered alarms are currently sounding.
	Triggered
)

// New creates a new Alarm.
func New(code string) *Alarm {
	a := &Alarm{
		State:      Disarmed,
		Code:       code,
		LowBeep:    func() {},
		MediumBeep: func() {},
		HighBeep:   func() {},
		StartAlarm: func() {},
		StopAlarm:  func() {},
		Logger:     func(format string, v ...interface{}) {},
	}
	a.Timeout = func(ctx context.Context) {
		for i := 10; i > 0; i-- {
			if ctx.Err() == context.Canceled {
				return
			}
			a.MediumBeep()
			a.Display = fmt.Sprintf("%d", i)
			time.Sleep(time.Second)
		}
	}
	return a
}

// Alarm which has door status
type Alarm struct {
	State State
	Code  string

	// Electronics interactions.
	LowBeep    func()
	MediumBeep func()
	HighBeep   func()
	StartAlarm func()
	StopAlarm  func()

	// buffer of pressed keys.
	buffer     string
	Failures   int
	doorIsOpen bool
	Display    string

	// Used to cancel timers.
	m             sync.Mutex
	cancellations []func()

	// Default timer.
	Timeout func(ctx context.Context)

	Logger func(format string, v ...interface{})
}

// KeyPressed is an event on the alarm.
func (a *Alarm) KeyPressed(key string) {
	if key == "*" {
		a.MediumBeep()
		a.backspace()
		a.Display = a.buffer
		return
	}
	if isDigit(key) {
		a.LowBeep()
	}
	if isLetter(key) {
		a.HighBeep()
	}
	if key == "C" {
		a.Logger("Clearing buffer")
		a.buffer = ""
		a.Display = a.buffer
		return
	}
	a.buffer += key
	a.Display = a.buffer
	if key == "#" {
		a.Logger("Attempting to execute command")
		a.MediumBeep()
		a.executeCommand()
		a.buffer = ""
	}
}

var alarmChangeRegexp = regexp.MustCompile(`B(\d+)B(\d+)#`)

func (a *Alarm) executeCommand() {
	// Examine the buffer for correct values.
	if strings.HasPrefix(a.buffer, "A") {
		// Arm the alarm.
		if a.buffer == "A"+a.Code+"#" {
			a.Logger("Arming the alarm")
			a.Arming()
		}
		return
	}
	if alarmChangeRegexp.MatchString(a.buffer) && a.State == Disarmed {
		m := alarmChangeRegexp.FindStringSubmatch(a.buffer)
		firstCode := m[1]
		if firstCode != a.Code {
			a.Logger("The entered code %v was not correct", firstCode)
			return
		}
		secondCode := m[2]
		a.Code = secondCode
		a.Logger("Changed the alarm code to %v", a.Code)
		a.LowBeep()
		a.MediumBeep()
		a.HighBeep()
		return
	}
	if strings.HasPrefix(a.buffer, "D"+a.Code+"#") {
		a.Logger("Disarming")
		a.Disarm()
		return
	}
}

// Disarm the alarm.
func (a *Alarm) Disarm() {
	a.m.Lock()
	defer a.m.Unlock()

	// Cancel the timers.
	for _, cancel := range a.cancellations {
		cancel()
	}
	a.cancellations = nil

	// Stop the alarm.
	a.StopAlarm()
	a.State = Disarmed
	a.Logger("Alarm disarmed")
	a.Display = "disa"
	go a.clearDisplayAfter(time.Second * 5)
}

// Arm the alarm.
func (a *Alarm) Arm() {
	a.State = Armed
	a.Logger("Armed")
	a.Display = "Armd"
	go a.clearDisplayAfter(time.Second * 5)
}

func (a *Alarm) clearDisplayAfter(d time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancellations = append(a.cancellations, cancel)
	select {
	case <-time.After(d):
		a.Display = ""
	case <-ctx.Done():
		return
	}
}

// Arming starts the arming process.
func (a *Alarm) Arming() {
	if a.State != Disarmed {
		a.Logger("Attempted to arm while state was not disarmed, current state is %v", a.State)
		return
	}
	a.State = Arming
	ctx, cancel := context.WithCancel(context.Background())
	a.cancellations = append(a.cancellations, cancel)
	go func() {
		a.Timeout(ctx)
		if ctx.Err() == context.Canceled {
			a.Logger("Alarm arming cancelled")
			return
		}
		a.Arm()
	}()
	return
}

// Triggering the alarm.
func (a *Alarm) Triggering() {
	a.Logger("Triggering alarm")
	a.State = Triggering
	ctx, cancel := context.WithCancel(context.Background())
	a.cancellations = append(a.cancellations, cancel)
	go func() {
		a.Timeout(ctx)
		if ctx.Err() == context.Canceled {
			a.Logger("Alarm triggering cancelled")
			return
		}
		a.Trigger()
	}()
	return
}

// Trigger the alarm.
func (a *Alarm) Trigger() {
	a.Logger("Alarm triggered")
	a.State = Triggered
	a.Display = "Alrm"
	a.StartAlarm()
}

func (a *Alarm) backspace() {
	if len(a.buffer) == 0 {
		return
	}
	a.buffer = a.buffer[:len(a.buffer)-1]
}

var digits = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

func isDigit(key string) bool {
	for _, d := range digits {
		if d == key {
			return true
		}
	}
	return false
}

var letters = []string{"A", "B", "C", "D"}

func isLetter(key string) bool {
	for _, d := range letters {
		if d == key {
			return true
		}
	}
	return false
}

// SetDoorIsOpen is used to set whether the door is open or not.
func (a *Alarm) SetDoorIsOpen(open bool) {
	if a.doorIsOpen == open {
		// No change.
		return
	}
	a.doorIsOpen = open
	if a.State == Armed && a.doorIsOpen {
		a.Logger("Triggering alarm due to door open")
		a.Triggering()
	}
	return
}
