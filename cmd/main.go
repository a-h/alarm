package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/a-h/alarm"
	"github.com/a-h/beeper"
	"github.com/a-h/keypad"
	"github.com/stianeikeland/go-rpio"
)

func main() {
	err := rpio.Open()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	defer rpio.Close()

	// Setup the keypad.
	p1 := rpio.Pin(4)
	p2 := rpio.Pin(17)
	p3 := rpio.Pin(27)
	p4 := rpio.Pin(22)
	p5 := rpio.Pin(18)
	p6 := rpio.Pin(23)
	p7 := rpio.Pin(24)
	p8 := rpio.Pin(25)

	log.Printf("Starting keypad...")
	pad := keypad.New(p1, p2, p3, p4, p5, p6, p7, p8)

	log.Printf("Creating alarm...")
	a := alarm.New("0000")

	// Setup the buzzer.
	log.Printf("Setting up buzzer...")
	buzzer := rpio.Pin(12)
	a.HighBeep = func() {
		beeper.Beep(buzzer, 880.00, time.Millisecond*50)
	}
	a.MediumBeep = func() {
		beeper.Beep(buzzer, 329.00, time.Millisecond*50)
	}
	a.LowBeep = func() {
		beeper.Beep(buzzer, 110.00, time.Millisecond*50)
	}
	a.LowBeep()
	a.MediumBeep()
	a.HighBeep()

	// Also use the buzzer for the alarm.
	log.Printf("Setting up alarm buzzer...")
	as := alarmSounder{
		Pin: buzzer,
	}
	a.StartAlarm = as.Start
	a.StopAlarm = as.Stop
	go as.Run()

	// Configure logging.
	a.Logger = log.Printf

	// Configure the reed switch.
	s := Debounce(rpio.Pin(21))

	log.Printf("Alarm state: %v", a.State)
	for {
		if keys, ok := pad.Read(); ok {
			for _, k := range keys {
				log.Printf("Key pressed: %v", k)
				a.KeyPressed(k)
			}
		}
		doorIsOpen := s() == rpio.Low
		a.SetDoorIsOpen(doorIsOpen)
	}
}

type alarmSounder struct {
	Pin        rpio.Pin
	isSounding bool
	m          sync.Mutex
}

func (as *alarmSounder) Start() {
	as.m.Lock()
	as.isSounding = true
	defer as.m.Unlock()
}
func (as *alarmSounder) Stop() {
	as.m.Lock()
	as.isSounding = false
	defer as.m.Unlock()
}

func (as *alarmSounder) Run() {
	for {
		if as.isSounding {
			beeper.Beep(as.Pin, 1000, time.Millisecond*50)
			time.Sleep(time.Millisecond * 10)
			beeper.Beep(as.Pin, 1500, time.Millisecond*50)
			time.Sleep(time.Millisecond * 10)
			beeper.Beep(as.Pin, 2000, time.Millisecond*50)
		}
		time.Sleep(time.Millisecond * 150)
	}
}

// Debounce a pin.
func Debounce(pin rpio.Pin) func() rpio.State {
	pin.PullDown()
	lastChange := time.Now()
	state := pin.Read()
	return func() rpio.State {
		if time.Now().Before(lastChange.Add(time.Millisecond * 10)) {
			return state
		}
		state = pin.Read()
		return state
	}
}
