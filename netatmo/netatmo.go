package netatmo

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const NETATMO_URL = "https://api.netatmo.com"
const DYNAMODB_NETATMO_TOKEN_DS = "netatmo_token"
const DYNAMODB_NETATMO_WEATHER_DS = "netatmo_weather"

type NetatmoApiToken struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresIn    int     `json:"expires_in"`
	Error        *string `json:"error"`
}

type NetatmoDashboardData struct {
	TimeUtc     int     `json:"time_utc"`
	Temperature float64 `json:"Temperature"`
	MinTemp     float64 `json:"min_temp"`
	MaxTemp     float64 `json:"max_temp"`
}

type NetatmoDevice struct {
	ModuleName    string               `json:"module_name"`
	DashboardData NetatmoDashboardData `json:"dashboard_data"`
}

type NetatmoBody struct {
	Devices []NetatmoDevice `json:"devices"`
}

type NetatmoApiResponse struct {
	Body  NetatmoBody `json:"body"`
	Error *string     `json:"error"`
}

type DynamoDBNetatmoToken struct {
	DS           string `json:"ds"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Expires      int    `json:"expires"`
	TS           int    `json:"ts"`
}

type DynamoDBNetatmoWeather struct {
	DS            string  `json:"ds"`
	TempInside    float64 `json:"temp_inside"`
	TempInsideMin float64 `json:"temp_inside_min"`
	TempInsideMax float64 `json:"temp_inside_max"`
	TS            int     `json:"ts"`
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

func FetchAccessToken() string {
	timestamp := time.Now().Unix()
	params := &dynamodb.QueryInput{
		TableName:              aws.String(os.Getenv("AWS_DYNAMODB_TABLE")),
		KeyConditionExpression: aws.String("ds = :ds AND ts < :ts"),
		FilterExpression:       aws.String("expires > :expires"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":ds":      {S: aws.String(DYNAMODB_NETATMO_TOKEN_DS)},
			":ts":      {N: aws.String(strconv.FormatInt(timestamp, 10))},
			":expires": {N: aws.String(strconv.FormatInt(timestamp, 10))},
		},
		ScanIndexForward: aws.Bool(false), // descending order
		Limit:            aws.Int64(1),
	}
	result, err := svc.Query(params)
	if err == nil && len(result.Items) != 0 {
		var item DynamoDBNetatmoToken
		err = dynamodbattribute.UnmarshalMap(result.Items[0], &item)
		if err != nil {
			log.Fatal(err)
		}
		return item.AccessToken
	}

	resp, err := http.PostForm(NETATMO_URL+"/oauth2/token", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {os.Getenv("NETATMO_REFRESH_TOKEN")},
		"client_id":     {os.Getenv("NETATMO_CLIENT_ID")},
		"client_secret": {os.Getenv("NETATMO_CLIENT_SECRET")},
	})
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var tokenData NetatmoApiToken
	json.Unmarshal(body, &tokenData)

	if tokenData.Error != nil {
		log.Fatal(tokenData.Error)
	}

	item := DynamoDBNetatmoToken{
		DS:           DYNAMODB_NETATMO_TOKEN_DS,
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
		Expires:      int(timestamp) + tokenData.ExpiresIn,
		TS:           int(timestamp),
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

	return tokenData.AccessToken
}

func Fetch() {
	token := FetchAccessToken()

	req, err := http.NewRequest("GET", NETATMO_URL+"/api/getstationsdata?device_id="+os.Getenv("NETATMO_DEVICE_ID"), nil)
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var apiResponse NetatmoApiResponse
	json.Unmarshal(body, &apiResponse)

	if apiResponse.Error != nil {
		log.Fatal(apiResponse.Error)
	}

	dashboardData := apiResponse.Body.Devices[0].DashboardData
	item := DynamoDBNetatmoWeather{
		DYNAMODB_NETATMO_WEATHER_DS,
		dashboardData.Temperature,
		dashboardData.MinTemp,
		dashboardData.MaxTemp,
		dashboardData.TimeUtc,
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
