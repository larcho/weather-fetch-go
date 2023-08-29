package api

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"weather-fetch-go/netatmo"
	"weather-fetch-go/weatherlink"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const DYNAMODB_NETATMO_DS = "netatmo_weather"
const DYNAMODB_WEATHERLINK_DS = "weatherlink_weather"

type ResponseTemperatureItem struct {
	Temp    float64 `json:"temp"`
	TempMax float64 `json:"temp_max"`
	TempMin float64 `json:"temp_min"`
	TS      int     `json:"timestamp"`
}

type ResponseRainItem struct {
	RainDaily float64 `json:"day"`
	RainRate  float64 `json:"rate"`
	TS        int     `json:"timestamp"`
}

type ResponseBody struct {
	Outside   *ResponseTemperatureItem `json:"outside"`
	Bedroom   *ResponseTemperatureItem `json:"bedroom"`
	KDKInside *ResponseTemperatureItem `json:"kdk_inside"`
	Rain      *ResponseRainItem        `json:"rain"`
	TS        int                      `json:"timestamp"`
}

var svc *dynamodb.DynamoDB

func init() {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_DEFAULT_REGION")),
	})
	if err != nil {
		log.Fatal(err)
	}
	svc = dynamodb.New(sess)
}

func ApiResponse() (events.APIGatewayProxyResponse, error) {

	responseBody := ResponseBody{
		TS: int(time.Now().Unix()),
	}

	location, err := time.LoadLocation("America/Asuncion")
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	now := time.Now().In(location)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	timestamp := startOfDay.Unix()

	// WeatherLink Data
	codeStartTime := time.Now()
	params := &dynamodb.QueryInput{
		TableName:              aws.String(os.Getenv("AWS_DYNAMODB_TABLE")),
		KeyConditionExpression: aws.String("ds = :ds AND ts >= :ts"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":ds": {S: aws.String(DYNAMODB_WEATHERLINK_DS)},
			":ts": {N: aws.String(strconv.FormatInt(timestamp, 10))},
		},
		ScanIndexForward: aws.Bool(true),
	}
	result, err := svc.Query(params)
	executionTime := time.Since(codeStartTime)
	log.Printf("Execution for fetching Weatherlink data from DyanmoDB took: %d milliseconds.", executionTime.Milliseconds())

	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	if len(result.Items) == 0 {
		return events.APIGatewayProxyResponse{}, nil
	}
	outsideMin := 100.0
	outsideMax := -100.0
	kdkMin := 100.0
	kdkMax := -100.0

	for index, resultItem := range result.Items {
		var item weatherlink.DynamoDBWeatherLink
		err = dynamodbattribute.UnmarshalMap(resultItem, &item)
		if err != nil {
			log.Fatal(err)
		}
		if item.TempOutside < outsideMin {
			outsideMin = item.TempOutside
		}
		if item.TempOutside > outsideMax {
			outsideMax = item.TempOutside
		}
		if item.TempInside < kdkMin {
			kdkMin = item.TempInside
		}
		if item.TempInside > kdkMax {
			kdkMax = item.TempInside
		}
		if index == len(result.Items)-1 {
			responseBody.Outside = &ResponseTemperatureItem{
				Temp:    (item.TempOutside - 32) * 5 / 9,
				TempMax: (outsideMax - 32) * 5 / 9,
				TempMin: (outsideMin - 32) * 5 / 9,
				TS:      item.TS,
			}
			responseBody.KDKInside = &ResponseTemperatureItem{
				Temp:    (item.TempInside - 32) * 5 / 9,
				TempMax: (kdkMax - 32) * 5 / 9,
				TempMin: (kdkMin - 32) * 5 / 9,
				TS:      item.TS,
			}
			responseBody.Rain = &ResponseRainItem{
				RainDaily: float64(item.RainDaily) * 0.2,
				RainRate:  float64(item.RainRate) * 0.2,
				TS:        item.TS,
			}
		}
	}

	// Netatmo Data
	timestamp = time.Now().Unix()

	codeStartTime = time.Now()

	params = &dynamodb.QueryInput{
		TableName:              aws.String(os.Getenv("AWS_DYNAMODB_TABLE")),
		KeyConditionExpression: aws.String("ds = :ds AND ts < :ts"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":ds": {S: aws.String(DYNAMODB_NETATMO_DS)},
			":ts": {N: aws.String(strconv.FormatInt(timestamp, 10))},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int64(1),
	}
	result, err = svc.Query(params)
	executionTime = time.Since(codeStartTime)
	log.Printf("Execution for fetching Netatmo data took: %d milliseconds.", executionTime.Milliseconds())
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	if len(result.Items) != 0 {
		var item netatmo.DynamoDBNetatmoWeather
		err = dynamodbattribute.UnmarshalMap(result.Items[0], &item)
		if err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		responseBody.Bedroom = &ResponseTemperatureItem{
			Temp:    item.TempInside,
			TempMax: item.TempInsideMax,
			TempMin: item.TempInsideMin,
			TS:      item.TS,
		}
	}

	responseBodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(responseBodyBytes),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func main() {
	lambda.Start(ApiResponse)
}
