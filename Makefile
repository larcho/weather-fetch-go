build:
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o netatmo/bootstrap netatmo/netatmo.go
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o weatherlink/bootstrap weatherlink/weatherlink.go
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o api/bootstrap api/api.go
