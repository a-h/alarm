package doorstatus

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"log"
	urlpkg "net/url"

	"github.com/a-h/alarm"
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"fmt"
)

// Adapted from https://github.com/eclipse/paho.mqtt.golang/blob/master/cmd/ssl/main.go
// Also see https://www.eclipse.org/paho/clients/golang/
func newTLSConfig(amazonRootCA1, certPEMBlock, keyPEMBlock []byte) (config *tls.Config, err error) {
	// Import trusted certificates from CAfile.pem.
	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(amazonRootCA1)

	// Import client certificate/key pair.
	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return
	}

	// Create tls.Config with desired tls properties
	config = &tls.Config{
		// RootCAs = certs used to verify server cert.
		RootCAs: certpool,
		// ClientAuth = whether to request cert from server.
		// Since the server is set up for SSL, this happens
		// anyways.
		ClientAuth: tls.NoClientCert,
		// ClientCAs = certs used to validate client cert.
		ClientCAs: nil,
		// Certificates = list of certs client sends to server.
		Certificates: []tls.Certificate{cert},
	}
	return
}

type iotResponse struct {
	State iotResponseState `json:"state"`
}

type iotResponseState struct {
	Desired  *Status `json:"desired"`
	Reported *Status `json:"reported"`
}

// Status of the device.
type Status struct {
	DoorIsOpen bool        `json:"doorIsOpen"`
	AlarmState alarm.State `json:"alarmState"`
}

// New create a new doorstatus updater.
// url should be in the form: tls://a3rmn7yfsg6nhl-ats.iot.eu-west-2.amazonaws.com:8883
func New(deviceName string, amazonRootCA1, certificatePEM, privatePEM []byte, url string, subscription chan<- alarm.State) (update chan Status, close func(), err error) {
	tlsconfig, err := newTLSConfig(amazonRootCA1, certificatePEM, privatePEM)
	if err != nil {
		err = fmt.Errorf("failed to create TLS configuration: %v", err)
		return
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(url)
	opts.SetClientID(deviceName).SetTLSConfig(tlsconfig)

	// Start the connection.
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		err = fmt.Errorf("failed to create connection: %v", token.Error())
		return
	}

	// Listen for updates on the channel and push them to IoT.
	update = make(chan Status, 10)
	go func() {
		for s := range update {
			s := s
			payload, err := json.Marshal(iotResponse{
				State: iotResponseState{
					Reported: &s,
				},
			})
			if err != nil {
				log.Printf("failed to marshal IoT response to JSON: %v", err)
				continue
			}
			token := c.Publish("$aws/things/"+urlpkg.PathEscape(deviceName)+"/shadow/update", 0, false, payload)
			token.Wait()
			if token.Error() != nil {
				log.Printf("failed to publish update: %v", token.Error())
			}
		}
	}()

	// Listen for events and publish them to the subscription channel.
	h := func(client mqtt.Client, msg mqtt.Message) {
		var r iotResponse
		err := json.Unmarshal(msg.Payload(), &r)
		if err != nil {
			log.Printf("failed to decode payload: %s\n", msg.Payload())
			return
		}
		if r.State.Desired != nil {
			subscription <- r.State.Desired.AlarmState
		}
	}
	if token := c.Subscribe("$aws/things/"+urlpkg.PathEscape(deviceName)+"/shadow/update/accepted", 0, h); token.Wait() && token.Error() != nil {
		err = fmt.Errorf("failed to create subscription: %v", token.Error())
		return
	}

	close = func() { c.Disconnect(250) }
	return
}
