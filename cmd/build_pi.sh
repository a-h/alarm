GOOS=linux GOARCH=arm GOARM=5 go build
scp ./cmd pi@192.168.0.45:/home/pi/alarm
scp ./*.pem* pi@192.168.0.45:/home/pi/
# /usr/local/bin/alarm -notify=tls://a3rmn7yfsg6nhl-ats.iot.eu-west-2.amazonaws.com:8883 -certificatePEM=4950ab3d29-certificate.pem.crt -privatePEM=4950ab3d29-private.pem.key &