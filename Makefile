build:
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o netatmo/bootstrap netatmo/main.go
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o weatherlink/bootstrap weatherlink/main.go
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o api/bootstrap api/main.go
