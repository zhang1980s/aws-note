build:
	if [ -f exportLog.zip ]; then rm -rf exportLog.zip; fi;
	if [ -f bootstrap ]; then rm bootstrap ; fi;
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap main.go;
	zip exportLog.zip bootstrap