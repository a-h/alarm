package doorstatus

import (
	"context"
	"net/http"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"
)

// New creates a new HomeKit switch.
func New(setArmedFromHomeKit chan<- bool) (updateArmedFromDevice chan bool, close func(), err error) {
	mux := http.NewServeMux()

	switchInfo := accessory.Info{
		Name: "Door Alarm",
	}
	acc := accessory.NewSwitch(switchInfo)

	config := hc.Config{Pin: "12341234", Port: "12345", StoragePath: "./db"}
	t, err := hc.NewIPTransport(config, acc.Accessory)
	if err != nil {
		log.Info.Panic(err)
	}

	// Listen for updates on the channel and push them to Homekit.
	updateArmedFromDevice = make(chan bool, 10)
	go func() {
		for isArmed := range updateArmedFromDevice {
			isArmed := isArmed
			log.Info.Printf("Switch turned on from Device: %v", isArmed)
			acc.Switch.On.SetValue(isArmed)
		}
	}()

	// Listen for events from HomeKit and publish them to the setArmedFromHomeKit channel.
	acc.Switch.On.OnValueRemoteUpdate(func(isArmed bool) {
		log.Info.Printf("Switch turned on from HomeKit: %v", isArmed)
		setArmedFromHomeKit <- isArmed
	})

	hc.OnTermination(func() {
		t.Start()
	})

	// Allow turning the switch on/off using the Web server
	mux.HandleFunc("/turnOn", func(w http.ResponseWriter, req *http.Request) {
		acc.Switch.On.SetValue(true)
		log.Info.Println("Armed")
		w.Write([]byte("Armed"))
	})

	mux.HandleFunc("/turnOff", func(w http.ResponseWriter, req *http.Request) {
		acc.Switch.On.SetValue(false)
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
