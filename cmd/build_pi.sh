GOOS=linux GOARCH=arm GOARM=5 go build
echo "built"
ssh pi@housealarmpi sudo killall alarm
echo "killed"
scp ./cmd "pi@housealarmpi:/home/pi/alarm"
echo "copied"
ssh pi@housealarmpi "sudo ./alarm"
echo "run"
