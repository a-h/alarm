package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"os/user"
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

var notifyFlag = flag.String("notify", "", "Set a URL to notify, e.g. tls://a3rmn7yfsg6nhl-ats.iot.eu-west-2.amazonaws.com:8883")
var certificatePEMFlag = flag.String("certificatePEM", "", "The certificate to use with AWS IoT")
var privatePEMFlag = flag.String("privatePEM", "", "The key to use with AWS IoT")

func main() {
	flag.Parse()
	if *notifyFlag == "" || *certificatePEMFlag == "" || *privatePEMFlag == "" {
		flag.PrintDefaults()
		return
	}

	var err error
	var u *user.User
	u, err = user.Current()
	if err != nil {
		log.Fatalf("Couldn't check if user is running as root: %v", err)
	}
	if u.Uid != "0" {
		log.Fatalf("The buzzer requires that the app is ran as root in order to use the PWM feature.")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	err = rpio.Open()
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
	a := alarm.New("0654")

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
	doorState, _ := s()
	log.Printf("Door initially open: %v", doorState == rpio.High)
	a.SetDoorIsOpen(doorState == rpio.High)

	// Create the IoT connection.
	certificatePEM, err := ioutil.ReadFile(*certificatePEMFlag)
	if err != nil {
		log.Fatalf("failed to read certificate file %s: %v", *certificatePEMFlag, err)
	}
	privatePEM, err := ioutil.ReadFile(*privatePEMFlag)
	if err != nil {
		log.Fatalf("failed to read private key file %s: %v", *privatePEMFlag, err)
	}
	subscription := make(chan alarm.State, 10)
	update, closer, err := doorstatus.New("alarm", []byte(amazonRootCA1), certificatePEM, privatePEM, *notifyFlag, subscription)
	if err != nil {
		log.Fatalf("failed to connect to IoT: %v", err)
	}

	// Send an initial status to IoT.
	log.Printf("Setting inital status")
	update <- doorstatus.Status{
		DoorIsOpen: doorState == rpio.High,
		AlarmState: a.State,
	}

	displaying := a.Display
	alarmState := a.State

exit:
	for {
		select {
		case sig := <-sigs:
			log.Printf("Shutdown signal received; %v", sig)
			break exit
		case state := <-subscription:
			log.Printf("Remote update received: %v", alarm.StateNames[state])
			switch state {
			case alarm.Armed:
				a.Arm()
			case alarm.Arming:
				a.Arming()
			case alarm.Disarmed:
				a.Disarm()
			case alarm.Triggered:
				a.Trigger()
			case alarm.Triggering:
				a.Triggering()
			}
		default:
			if keys, ok := pad.Read(); ok {
				for _, k := range keys {
					log.Printf("Key pressed: %v", k)
					a.KeyPressed(k)
				}
			}

			var notificationRequired bool

			// If the door state has changed, send a notification.
			var doorStateUpdated bool
			if doorState, doorStateUpdated = s(); doorStateUpdated {
				log.Printf("Door open: %v", doorState == rpio.High)
				a.SetDoorIsOpen(doorState == rpio.High)
				notificationRequired = true
			}

			// If the alarm state has changed, send a notification.
			if alarmState != a.State {
				notificationRequired = true
				alarmState = a.State
			}

			if notificationRequired {
				log.Printf("Updating status")
				update <- doorstatus.Status{
					DoorIsOpen: doorState == rpio.High,
					AlarmState: a.State,
				}
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
	close(update)
	closer()
	log.Printf("Shutdown complete")
}

func firstFourCharacters(s string) string {
	if len(s) > 4 {
		return s[len(s)-4:]
	}
	return s
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
