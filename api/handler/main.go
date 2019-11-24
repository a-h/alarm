package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/a-h/alarm"
	"github.com/akrylysov/algnhsa"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iotdataplane"
)

func main() {
	algnhsa.ListenAndServe(http.HandlerFunc(handle), nil)
}

type iotResponse struct {
	State iotResponseState `json:"state"`
}

type iotResponseState struct {
	Desired status `json:"desired"`
}

type status struct {
	AlarmState alarm.State `json:"alarmState"`
}

func handle(w http.ResponseWriter, r *http.Request) {
	// Decode request.
	var update iotResponse
	d := json.NewDecoder(r.Body)
	err := d.Decode(&update.State.Desired)
	if err != nil {
		log.Printf("could not decode JSON: %v", err)
		http.Error(w, "could not decode JSON", http.StatusUnprocessableEntity)
		return
	}

	// Encode the update.
	payload, err := json.Marshal(update)
	if err != nil {
		log.Printf("could not encode JSON: %v", err)
		http.Error(w, "could not encode JSON", http.StatusInternalServerError)
		return
	}

	// Send it to the IoT endpoint.
	sess := session.Must(session.NewSession())
	endpoint := os.Getenv("IOT_ENDPOINT")
	conf := aws.NewConfig().WithEndpoint(endpoint)
	iot := iotdataplane.New(sess, conf)

	_, err = iot.UpdateThingShadow(&iotdataplane.UpdateThingShadowInput{
		ThingName: aws.String(os.Getenv("IOT_THINGNAME")),
		Payload:   []byte(payload),
	})
	if err != nil {
		log.Printf("error updating thing: %v", err)
		http.Error(w, "error updating thing", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(`{ "status": "ok" }`))
}
