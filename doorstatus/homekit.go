package doorstatus

import (
	"context"
	"net/http"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/log"
)

// New creates a new HomeKit alarm.
func New(setArmedFromHomeKit chan<- bool) (updateArmedFromDevice chan bool, updateDoorIsOpenFromDevice chan bool, close func(), err error) {
	mux := http.NewServeMux()

	bridge := accessory.NewBridge(accessory.Info{
		ID:   1,
		Name: "Door Alarm",
	})
	alarmAccessory := accessory.NewSwitch(accessory.Info{
		ID:   2,
		Name: "Armed",
	})
	doorAccessory := accessory.NewSwitch(accessory.Info{
		ID:   3,
		Name: "Open",
	})
	doorAccessory.Switch.Characteristics[0].Perms = []string{
		characteristic.PermRead,
		characteristic.PermEvents,
	}
	config := hc.Config{Pin: "12341234", Port: "12345", StoragePath: "./db"}
	t, err := hc.NewIPTransport(config, bridge.Accessory, doorAccessory.Accessory, alarmAccessory.Accessory)
	if err != nil {
		log.Info.Panic(err)
	}

	// Listen for updates on the channel and push them to Homekit.
	updateArmedFromDevice = make(chan bool, 10)
	updateDoorIsOpenFromDevice = make(chan bool, 10)
	var isOpen bool
	go func() {
		for {
			select {
			case isArmed := <-updateArmedFromDevice:
				log.Info.Printf("Switch turned on from Device: %v", isArmed)
				alarmAccessory.Switch.On.SetValue(isArmed)
				break
			case isOpen = <-updateDoorIsOpenFromDevice:
				log.Info.Printf("Setting door open in HomeKit: %v", isOpen)
				doorAccessory.Switch.On.SetValue(isOpen)
				log.Info.Printf("Set door value open in HomeKit: %v", isOpen)
				break
			}
		}
	}()

	// Listen for events from HomeKit and publish them to the setArmedFromHomeKit channel.
	alarmAccessory.Switch.On.OnValueRemoteUpdate(func(isArmed bool) {
		log.Info.Printf("Alarm armed from HomeKit - isArmed: %v", isArmed)
		setArmedFromHomeKit <- isArmed
	})

	hc.OnTermination(func() {
		t.Start()
	})

	// Allow turning the switch on/off using the Web server
	mux.HandleFunc("/turnOn", func(w http.ResponseWriter, req *http.Request) {
		alarmAccessory.Switch.On.SetValue(true)
		log.Info.Println("Armed")
		w.Write([]byte("Armed"))
	})

	mux.HandleFunc("/turnOff", func(w http.ResponseWriter, req *http.Request) {
		alarmAccessory.Switch.On.SetValue(false)
		log.Info.Println("Disarmed")
		w.Write([]byte("Disarmed"))
	})

	// Start up the Web server.
	s := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}
	go func() {
		log.Debug.Println("Starting server...")
		err = s.ListenAndServe()
		if err != nil {
			log.Debug.Println("Error listening to server", err)
		}
	}()
	go func() {
		t.Start()
	}()
	close = func() {
		t.Stop()
		s.Shutdown(context.Background())
	}
	return
}
