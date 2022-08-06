package iot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"log"

	"github.com/a-h/alarm"
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
func New(controlAlarmFromIoT chan<- alarm.State, code *string) (updateStateFromDevice chan alarm.State, updateDoorIsOpenFromDevice chan bool, close func(), err error) {
	// Listen for updates on the channels.
	updateStateFromDevice = make(chan alarm.State, 10)
	updateDoorIsOpenFromDevice = make(chan bool, 10)

	var isOpen bool
	var deviceStatus alarm.State

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
		log.Printf("Received message: %s on topic: %s", msg.Payload(), msg.Topic())
		var alarmMessage AlarmMessage
		json.Unmarshal(msg.Payload(), &alarmMessage)
		if alarmMessage.Code == *code {
			switch alarmMessage.Action {
			case "ARM_HOME":
				controlAlarmFromIoT <- alarm.Arming
			case "ARM_AWAY":
				controlAlarmFromIoT <- alarm.Arming
			case "DISARM":
				controlAlarmFromIoT <- alarm.Disarmed
			case "TRIGGER":
				controlAlarmFromIoT <- alarm.Triggered
			}
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
	publishAvailable(client)

	// Every 10 minutes, publish the current state.
	ticker := time.NewTicker(10 * time.Minute)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Printf("Ticker: Publishing current state")
				publishAvailable(client)
				publishAlarm(client, deviceStatus)
				publishDoor(client, isOpen)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case deviceStatus = <-updateStateFromDevice:
				publishAvailable(client)
				publishAlarm(client, deviceStatus)
			case isOpen = <-updateDoorIsOpenFromDevice:
				publishDoor(client, isOpen)
				publishAvailable(client)
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
	log.Printf("Subscribed to topic %s", topic)
}

func publishDoor(client mqtt.Client, isOpen bool) {
	log.Printf("Setting door value in MQTT: %v", isOpen)
	if isOpen {
		publish(client, "home-assistant/door/contact", 0, "payload_on", true)
	} else {
		publish(client, "home-assistant/door/contact", 0, "payload_off", true)
	}
}

func publishAlarm(client mqtt.Client, deviceStatus alarm.State) {
	log.Printf("Setting alarm value in MQTT: %v", deviceStatus)
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
}

func publishAvailable(client mqtt.Client) {
	publish(client, "home-assistant/alarm/availability", 1, "online", false)
	publish(client, "home-assistant/door/availability", 1, "online", false)
}
