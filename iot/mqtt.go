package iot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/a-h/alarm"
	"github.com/brutella/hc/log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type AlarmMessage struct {
	Action string `json:"action"`
	Code   string `json:"code"`
}

type Credentials struct {
	Username string `json:"user"`
	Password string `json:"pass"`
	Broker   string `json:"broker"`
	Port     int    `json:"port"`
}

// New creates a new IoT alarm using MQTT.
func New(controlAlarmFromIoT chan<- bool, code *string) (updateStateFromDevice chan alarm.State, updateDoorIsOpenFromDevice chan bool, close func(), err error) {
	// Listen for updates on the channels.
	updateStateFromDevice = make(chan alarm.State, 10)
	updateDoorIsOpenFromDevice = make(chan bool, 10)
	var isOpen bool

	// Read the credentials.
	creds_data, err := ioutil.ReadFile("./creds.json")
	if err != nil {
		return nil, nil, nil, err
	}
	var creds Credentials
	err = json.Unmarshal(creds_data, &creds)

	// Create the MQTT options.
	options := mqtt.NewClientOptions()
	options.AddBroker(fmt.Sprintf("tcp://%s:%d", creds.Broker, creds.Port))
	options.SetClientID("go_mqtt_client")
	options.SetUsername(creds.Username)
	options.SetPassword(creds.Password)
	options.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		// Runs when a message that is subscribed to is received.
		var alarmMessage AlarmMessage
		json.Unmarshal(msg.Payload(), &alarmMessage)
		if alarmMessage.Code == *code {
			controlAlarmFromIoT <- alarmMessage.Action != "DISARM"
		}
	})

	// Create the MQTT client.
	client := mqtt.NewClient(options)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// Subscribe to the alarm topic.
	subscribe(client, "home-assistant/alarm/control", 1)

	// Publish the availability topic.
	publish(client, "home-assistant/alarm/availability", 1, "online", false)
	publish(client, "home-assistant/door/availability", 1, "online", false)

	go func() {
		for {
			select {
			case deviceStatus := <-updateStateFromDevice:
				log.Info.Printf("Setting alarm value in MQTT: %v", deviceStatus)
				switch deviceStatus {
				case alarm.Disarmed:
					publish(client, "home-assistant/alarm/contact", 1, "disarmed", true)
				case alarm.Armed:
					publish(client, "home-assistant/alarm/contact", 1, "armed_home", true)
				case alarm.Triggering:
					publish(client, "home-assistant/alarm/contact", 1, "pending", true)
				case alarm.Triggered:
					publish(client, "home-assistant/alarm/contact", 1, "triggered", true)
				case alarm.Arming:
					publish(client, "home-assistant/alarm/contact", 1, "arming", true)
				}
			case isOpen = <-updateDoorIsOpenFromDevice:
				log.Info.Printf("Setting door value in MQTT: %v", isOpen)
				if isOpen {
					publish(client, "home-assistant/door/contact", 0, "payload_on", true)
				} else {
					publish(client, "home-assistant/door/contact", 0, "payload_off", true)
				}
			}

		}
	}()
	return
}

func publish(client mqtt.Client, topic string, qos byte, payload string, retain bool) {
	token := client.Publish(topic, qos, retain, payload)
	token.Wait()
}

func subscribe(client mqtt.Client, topic string, qos byte) {
	token := client.Subscribe(topic, qos, nil)
	token.Wait()
	log.Info.Printf("Subscribed to topic %s", topic)
}
