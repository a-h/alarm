package alarm

import (
	"testing"
)

func TestButtons(t *testing.T) {
	actualCode := "1234"
	tests := []struct {
		name               string
		inputs             string
		expectedLowBeeps   int
		expectedMedBeeps   int
		expectedHighBeeps  int
		startState         State
		expectedState      State
		expectedBuffer     string
		expectedFailures   int
		expectedStopAlarms int
	}{
		{
			name:             "when a digit is entered, it does a low beep",
			inputs:           "1",
			expectedLowBeeps: 1,
			expectedBuffer:   "1",
		},
		{
			name:              "if a letter is pressed, it does a high beep",
			inputs:            "A",
			expectedHighBeeps: 1,
			expectedBuffer:    "A",
		},
		{
			name:              "when inputs are entered, they're added to a buffer",
			inputs:            "A1",
			expectedLowBeeps:  1,
			expectedHighBeeps: 1,
			expectedBuffer:    "A1",
		},
		{
			name:             "when a * is entered, the last character is removed and a medium beep happens",
			inputs:           "12*",
			expectedLowBeeps: 2,
			expectedMedBeeps: 1,
			expectedBuffer:   "1",
		},
		{
			name:             "when * is entered and the buffer is empty, a medium beep happens",
			inputs:           "*",
			expectedMedBeeps: 1,
			expectedBuffer:   "",
		},
		{
			name:              "entering A, plus the code, then # executes the command",
			inputs:            "A1234#",
			expectedHighBeeps: 1,
			expectedLowBeeps:  4,
			expectedMedBeeps:  1,
			expectedBuffer:    "",
			expectedState:     Arming,
		},
		{
			name:              "pressing C empties the buffer",
			inputs:            "A12C",
			expectedHighBeeps: 2,
			expectedLowBeeps:  2,
			expectedBuffer:    "",
			expectedState:     Disarmed,
		},
		{
			name:               "if the alarm is triggering, it can be disarmed by entering the disarm code",
			inputs:             "D1234#",
			startState:         Triggering,
			expectedHighBeeps:  1,
			expectedLowBeeps:   4,
			expectedMedBeeps:   1,
			expectedBuffer:     "",
			expectedStopAlarms: 1,
			expectedState:      Disarmed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actualLowBeeps, actualMedBeeps, actualHighBeeps, actualStopAlarms int
			alarm := New(actualCode)
			alarm.State = test.startState
			alarm.LowBeep = func() {
				actualLowBeeps++
			}
			alarm.MediumBeep = func() {
				actualMedBeeps++
			}
			alarm.HighBeep = func() {
				actualHighBeeps++
			}
			alarm.StopAlarm = func() {
				actualStopAlarms++
			}
			for _, key := range test.inputs {
				alarm.KeyPressed(string(key))
			}
			if alarm.State != test.expectedState {
				t.Errorf("expected state: %v, got %v", test.expectedState, alarm.State)
			}
			if alarm.buffer != test.expectedBuffer {
				t.Errorf("expected buffer: %q, got %q", test.expectedBuffer, alarm.buffer)
			}
			if actualLowBeeps != test.expectedLowBeeps {
				t.Errorf("expected low beeps: %d, got %d", test.expectedLowBeeps, actualLowBeeps)
			}
			if actualMedBeeps != test.expectedMedBeeps {
				t.Errorf("expected med beeps: %d, got %d", test.expectedMedBeeps, actualMedBeeps)
			}
			if actualHighBeeps != test.expectedHighBeeps {
				t.Errorf("expected high beeps: %d, got %d", test.expectedHighBeeps, actualHighBeeps)
			}
			if alarm.Failures != test.expectedFailures {
				t.Errorf("expected failures: %d, got: %d", test.expectedFailures, alarm.Failures)
			}
			if actualStopAlarms != test.expectedStopAlarms {
				t.Errorf("expected alarm stops: %d, got: %d", test.expectedStopAlarms, actualStopAlarms)
			}
		})
	}
}

func TestDoorOpened(t *testing.T) {
	tests := []struct {
		name                string
		start               State
		inputs              []bool
		expectedState       State
		expectedAlarmStarts int
		expectedAlarmStops  int
	}{
		{
			name:                "no change == no action",
			start:               Armed,
			inputs:              []bool{false, false, false, false},
			expectedState:       Armed,
			expectedAlarmStarts: 0,
			expectedAlarmStops:  0,
		},
		{
			name:                "when the alarm is armed, opening the door triggers the alarm",
			start:               Armed,
			inputs:              []bool{false, false, true},
			expectedState:       Triggering,
			expectedAlarmStarts: 0,
			expectedAlarmStops:  0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actualAlarmStarts, actualAlarmStops int
			alarm := New("1234")
			alarm.State = test.start
			alarm.StartAlarm = func() {
				actualAlarmStarts++
			}
			alarm.StopAlarm = func() {
				actualAlarmStops++
			}
			for _, doorState := range test.inputs {
				alarm.SetDoorIsOpen(doorState)
			}
			if alarm.State != test.expectedState {
				t.Errorf("expected state: %v, got %v", test.expectedState, alarm.State)
			}
			if actualAlarmStarts != test.expectedAlarmStarts {
				t.Errorf("expected alarm starts: %v, got %v", test.expectedAlarmStarts, actualAlarmStarts)
			}
			if actualAlarmStops != test.expectedAlarmStops {
				t.Errorf("expected alarm stops: %v, got %v", test.expectedAlarmStops, actualAlarmStops)
			}
		})
	}
}

func TestAlarmCodeChange(t *testing.T) {
	alarm := New("1234")
	for _, k := range "B4321B4321#" {
		alarm.KeyPressed(string(k))
	}
	if alarm.Code != "1234" {
		t.Errorf("should not possible to change the code without entering the correct code first")
	}
	for _, k := range "B1234B4321#" {
		alarm.KeyPressed(string(k))
	}
	if alarm.Code != "4321" {
		t.Errorf("expected the sequence of keys to change the code")
	}
}
