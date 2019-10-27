package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/a-h/alarm/doorstatus"

	"github.com/a-h/segment"

	"github.com/a-h/alarm"
	"github.com/a-h/beeper"
	"github.com/a-h/keypad"
	"github.com/stianeikeland/go-rpio"
)

var notifyFlag = flag.String("notify", "", "Set a URL to notify")

func main() {
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

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

	// Setup the display.
	// Row 1 (top row of pins).
	pD1 := rpio.Pin(8)
	pa := rpio.Pin(7)
	pf := rpio.Pin(16)
	pD2 := rpio.Pin(20)
	pD3 := rpio.Pin(26)
	pb := rpio.Pin(19)
	// Row 2 (bottom row of pins).
	pe := rpio.Pin(13)
	pd := rpio.Pin(6)
	pdp := rpio.Pin(5)
	pc := rpio.Pin(0)
	pg := rpio.Pin(11)
	pD4 := rpio.Pin(9)

	disp := segment.NewFourDigitSevenSegmentDisplay(pD1, pa, pf, pD2, pD3, pb, pe, pd, pdp, pc, pg, pD4)

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

	// Configure whether to send events to the Internet.
	netSw := Debounce(rpio.Pin(2))

	// Configure the reed switch.
	s := Debounce(rpio.Pin(21))
	doorState, _ := s()
	log.Printf("Door initially open: %v", doorState == rpio.High)
	a.SetDoorIsOpen(doorState == rpio.High)

	// Create a channel for the notifications to go to.
	eventNotifications := make(chan doorstatus.Event, 10)
	// Send a notification to the queue.
	eventNotifications <- doorstatus.EventAlarmStarted
	eventNotifications <- openOrClosedEvent(doorState == rpio.High)
	go func() {
		dsu := doorstatus.NewUpdater(*notifyFlag)
		shouldNotify := *notifyFlag != ""
		for e := range eventNotifications {
			if !shouldNotify {
				log.Printf("Skipping notification, because notify flag was not set: %v", e)
				continue
			}
			log.Printf("Sending notification: %v", e)
			err := dsu(e)
			if err != nil {
				log.Printf("Error posting status: %v", err)
				continue
			}
			log.Print("Notification sent OK.")
		}
	}()

	displaying := a.Display
	alarmState := a.State

exit:
	for {
		select {
		case sig := <-sigs:
			log.Printf("Shutdown signal received; %v", sig)
			break exit
		default:
			if keys, ok := pad.Read(); ok {
				for _, k := range keys {
					log.Printf("Key pressed: %v", k)
					a.KeyPressed(k)
				}
			}
			if doorState, updated := s(); updated {
				doorIsOpen := doorState == rpio.High
				log.Printf("Door open: %v", doorIsOpen)
				a.SetDoorIsOpen(doorIsOpen)
				// Send a notification to the queue.
				if sw, _ := netSw(); sw == rpio.Low {
					log.Printf("Internet switch ON, sending notification.")
					eventNotifications <- openOrClosedEvent(doorIsOpen)
				} else {
					log.Printf("Internet switch OFF, skipping notification.")
				}
			}

			// If the state has changed, send a notification.
			if alarmState != a.State {
				if e, ok := stateToEvent[a.State]; ok {
					eventNotifications <- e
				}
				alarmState = a.State
			}

			// Update the display.
			toDisplay := firstFourCharacters(a.Display)
			if displaying != toDisplay {
				log.Printf("Updating screen! %s", toDisplay)
				displaying = toDisplay
			}
			disp.Update(displaying)
			disp.Render()
		}
	}
	close(eventNotifications)
	log.Printf("Shutdown complete")
}

func firstFourCharacters(s string) string {
	if len(s) > 4 {
		return s[len(s)-4:]
	}
	return s
}

var stateToEvent = map[alarm.State]doorstatus.Event{
	alarm.Armed:     doorstatus.EventAlarmArmed,
	alarm.Disarmed:  doorstatus.EventAlarmDisarmed,
	alarm.Triggered: doorstatus.EventAlarmTriggered,
}

func openOrClosedEvent(isOpen bool) doorstatus.Event {
	if isOpen {
		return doorstatus.EventDoorOpened
	}
	return doorstatus.EventDoorClosed
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
func Debounce(pin rpio.Pin) func() (s rpio.State, updated bool) {
	pin.PullUp()
	lastChange := time.Now()
	state := pin.Read()
	return func() (rpio.State, bool) {
		if time.Now().Before(lastChange.Add(time.Millisecond * 10)) {
			return state, false
		}
		prev := state
		state = pin.Read()
		return state, prev != state
	}
}
