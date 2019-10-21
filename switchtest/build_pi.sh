GOOS=linux GOARCH=arm GOARM=5 go build
scp ./switchtest pi@192.168.0.45:/home/pi/switchtest