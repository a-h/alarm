package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio"
)

var pinFlag = flag.Int("pin", 2, "the BCM pin to use")

func main() {
	flag.Parse()

	err := rpio.Open()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	defer rpio.Close()

	fmt.Printf("Pin %d\n", *pinFlag)

	// Configure the switch.
	s := Debounce(rpio.Pin(*pinFlag))
	state, _ := s()
	fmt.Printf("On: %v\n", state == rpio.Low)

	for {
		state, updated := s()
		if updated {
			fmt.Printf("On: %v\n", state == rpio.Low)
		}
		time.Sleep(time.Millisecond * 100)
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
