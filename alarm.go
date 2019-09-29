package alarm

import (
	"context"
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
	a.Timeout = func() {
		for i := 0; i < 10; i++ {
			a.MediumBeep()
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
	// Buffer of pressed keys.
	Buffer     string
	Failures   int
	doorIsOpen bool

	// Used to cancel timers.
	m             sync.Mutex
	cancellations []func()

	// Default timer.
	Timeout func()

	Logger func(format string, v ...interface{})
}

// KeyPressed is an event on the alarm.
func (a *Alarm) KeyPressed(key string) {
	if key == "*" {
		a.MediumBeep()
		a.backspace()
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
		a.Buffer = ""
		return
	}
	a.Buffer += key
	if key == "#" {
		a.Logger("Attempting to execute command")
		a.MediumBeep()
		a.executeCommand()
		a.Buffer = ""
	}
}

var alarmChangeRegexp = regexp.MustCompile(`B\d+B(\d+)#`)

func (a *Alarm) executeCommand() {
	// Examine the buffer for correct values.
	if strings.HasPrefix(a.Buffer, "A") {
		// Arm the alarm.
		if a.Buffer == "A"+a.Code+"#" {
			a.Logger("Arming the alarm")
			//TODO: This should actually transition to arming and have a timeout.
			a.Arm()
		}
		return
	}
	if alarmChangeRegexp.MatchString(a.Buffer) && a.State == Disarmed {
		//TODO: Check that the codes match correct.
		a.Code = alarmChangeRegexp.FindStringSubmatch(a.Buffer)[1]
		a.Logger("Changed the alarm code to %v", a.Code)
		return
	}
	if strings.HasPrefix(a.Buffer, "D"+a.Code+"#") {
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
	a.State = Disarmed
	a.StopAlarm()
	a.Logger("Alarm stopped")
}

// Arm the alarm.
func (a *Alarm) Arm() {
	a.State = Armed
}

// Triggering the alarm.
func (a *Alarm) Triggering() {
	a.State = Triggering
	ctx, cancel := context.WithCancel(context.Background())
	a.cancellations = append(a.cancellations, cancel)
	go func() {
		a.Timeout()
		if ctx.Err() == context.Canceled {
			a.Logger("Alarm triggering cancelled")
			return
		}
		a.Logger("Triggering alarm")
		a.Trigger()
	}()
	return
}

// Trigger the alarm.
func (a *Alarm) Trigger() {
	a.State = Triggered
	a.StartAlarm()
}

func (a *Alarm) backspace() {
	if len(a.Buffer) == 0 {
		return
	}
	a.Buffer = a.Buffer[:len(a.Buffer)-1]
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
		a.Triggering()
	}
	return
}
