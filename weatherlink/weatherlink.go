package weatherlink

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const DYNAMODB_WEATHERLINK_DS = "weatherlink_weather"

type WeatherLinkCondition struct {
	DataStructureType int      `json:"data_structure_type"`
	Txid              int      `json:"txid"`
	Lsid              int      `json:"lsid"`
	TempInside        *float64 `json:"temp_in,omitempty"`
	Temp              *float64 `json:"temp,omitempty"`
	RainDaily         *int     `json:"rainfall_daily,omitempty"`
	RainRate          *int     `json:"rain_rate_last,omitempty"`
}

type WeatherLinkData struct {
	Did        string                 `json:"did"`
	TS         int                    `json:"ts"`
	Conditions []WeatherLinkCondition `json:"conditions"`
}

type WeatherLinkApiResponse struct {
	Data  WeatherLinkData `json:"data"`
	Error string          `json:"error"`
}

type DynamoDBWeatherLink struct {
	DS          string  `json:"ds"`
	TempOutside float64 `json:"temp_outside"`
	TempInside  float64 `json:"temp_inside"`
	RainDaily   int     `json:"rain_daily"`
	RainRate    int     `json:"rain_rate"`
	TS          int     `json:"ts"`
}

func Fetch() {
	resp, err := http.Get(os.Getenv("WEATHERLINK_URL"))
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var data WeatherLinkApiResponse
	json.Unmarshal(body, &data)

	var tempOutside float64
	var tempInside float64
	var rainDaily int
	var rainRate int
	for _, condition := range data.Data.Conditions {
		switch condition.Lsid {
		case 384563:
			tempOutside = *condition.Temp
			rainDaily = *condition.RainDaily
			rainRate = *condition.RainRate
		case 276340:
			tempInside = *condition.TempInside
		}
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_DEFAULT_REGION")),
	})

	if err != nil {
		log.Fatal(err)
	}

	svc := dynamodb.New(sess)

	item := DynamoDBWeatherLink{
		DYNAMODB_WEATHERLINK_DS,
		tempOutside,
		tempInside,
		rainDaily,
		rainRate,
		data.Data.TS,
	}
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		log.Fatal(err)
	}
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(os.Getenv("AWS_DYNAMODB_TABLE")),
	}
	_, err = svc.PutItem(input)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	lambda.Start(Fetch)
}
