package doorstatus

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// NewUpdater requires an input URL, e.g. https://sdyfuistbfn.execute-api.eu-west-1.amazonaws.com/dev/sensor/detect
// and will post data to the API.
func NewUpdater(url string) func(open bool) error {
	http.DefaultClient.Timeout = time.Second * 30
	return func(isOpen bool) error {
		var r *bytes.Buffer
		if isOpen {
			r = bytes.NewBufferString(`{ "event": "door_opened" }`)
		} else {
			r = bytes.NewBufferString(`{ "event": "door_closed" }`)
		}
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
