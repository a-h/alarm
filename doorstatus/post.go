package doorstatus

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// Event type.
type Event string

// EventDoorOpened has the value "door_opened".
const EventDoorOpened Event = "door_opened"

// EventDoorClosed has the value "door_closed".
const EventDoorClosed Event = "door_closed"

// EventAlarmStarted has the value "alarm_started".
const EventAlarmStarted Event = "alarm_started"

// EventAlarmArmed has the value "alarm_armed".
const EventAlarmArmed Event = "alarm_armed"

// EventAlarmDisarmed has the value "alarm_disarmed".
const EventAlarmDisarmed Event = "alarm_disarmed"

// EventAlarmTriggered has the value "alarm_triggered".
const EventAlarmTriggered Event = "alarm_triggered"

// NewUpdater requires an input URL, e.g. https://sdyfuistbfn.execute-api.eu-west-1.amazonaws.com/dev/sensor/detect
// and will post data to the API.
func NewUpdater(url string) func(Event) error {
	http.DefaultClient.Timeout = time.Second * 30
	return func(event Event) error {
		r := bytes.NewBufferString(fmt.Sprintf(`{ "event": %q }`, event))
		resp, err := http.Post(url, "application/json", r)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code %v from doorstatus API", resp.StatusCode)
		}
		return nil
	}
}
