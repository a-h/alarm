build:
	GOOS=linux GOARCH=arm GOARM=5 go build cmd/main.go
deploy: build
	ssh pi@housealarmpi sudo systemctl stop alarm
	scp ./iot/creds.json pi@housealarmpi:/home/pi/creds.json
	scp ./main "pi@housealarmpi:/home/pi/alarm"
	ssh pi@housealarmpi sudo systemctl start alarm