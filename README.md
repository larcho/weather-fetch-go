# Weather Fetch Go

My father and I are somewhat of a weather-station aficionado. We have a couple of stations at home, one from WeatherLink and another from Netatmo. Being an Apple Watch user I developed a simple app that fetches the weather information displaying the data within an App and some complication options.

A few years ago, I initially built this API using PHP. Being interested in learning more about Golang and AWS SAM, I thought it would be a good idea to rewrite this simple REST API using Golang and SAM.

## Setup DyamoDB

Create a new DynamoDB table with a partition key named ds (Data Structure) and a sorting key named ts (Timestamp).

## Build the Go App

```shell
make build
```

## Build and deploy SAM

```shell
sam build
sam deploy --guided
```

## TODO

- DyamoDB setup using the SAM template.
- Unit and integration tests.
