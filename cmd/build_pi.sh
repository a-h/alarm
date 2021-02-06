GOOS=linux GOARCH=arm GOARM=5 go build
ssh pi@housealarmpi sudo pkill alarm

scp ./cmd pi@housealarmpi:/home/pi/alarm
scp ./*.pem* pi@housealarmpi:/home/pi/

ssh pi@housealarmpi sudo ./alarm -notify=tls://a3rmn7yfsg6nhl-ats.iot.eu-west-2.amazonaws.com:8883 -certificatePEM=6076f830be-certificate.pem.crt -privatePEM=6076f830be-private.pem.key </dev/null &>/dev/null &
# To install...
# Copy the certificates and binary to the local bin directory.
# sudo cp *.pem* /usr/local/bin
# sudo cp alarm /usr/local/bin

# Update /etc/rc.local to run the alarm on startup.
# /usr/local/bin/alarm -notify=tls://a3rmn7yfsg6nhl-ats.iot.eu-west-2.amazonaws.com:8883 -certificatePEM=6076f830be-certificate.pem.crt -privatePEM=6076f830be-private.pem.key &

# Remember to kill the alarm before running it locally.
# sudo kill all alarm

# Run locally
# sudo ./alarm -notify=tls://a3rmn7yfsg6nhl-ats.iot.eu-west-2.amazonaws.com:8883 -certificatePEM=6076f830be-certificate.pem.crt -privatePEM=6076f830be-private.pem.key