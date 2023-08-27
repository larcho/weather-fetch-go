package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
  "net/url"
  "os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/google/uuid"
)

const NETATMO_URL = "https://api.netatmo.com"

type NetatmoApiToken struct {
  AccessToken  string  `json:"access_token"`
  RefreshToken string  `json:"refresh_token"`
  ExpiresIn    int     `json:"expires_in"`
  Error        *string `json:"error"`
}

type NetatmoDashboardData struct {
  TimeUtc     int `json:"time_utc"`
  Temperature float64 `json:"Temperature"`
  MinTemp     float64 `json:"min_temp"`
  MaxTemp     float64 `json:"max_temp"`
}

type NetatmoDevice struct {
  ModuleName    string `json:"module_name"`
  DashboardData NetatmoDashboardData `json:"dashboard_data"`
}

type NetatmoBody struct {
  Devices []NetatmoDevice `json:"devices"`
}

type NetatmoApiResponse struct {
  Body NetatmoBody `json:"body"`
  Error        *string `json:"error"`
}

type DynamoDBNetatmo struct {
  ID            string `json:"id"`
  TempInside    float64 `json:"temp_inside"`
  TempInsideMin float64 `json:"temp_inside_min"`
  TempInsideMax float64 `json:"temp_inside_max"`
  TS            int `json:"ts"`
}

func main() {
  respToken, err := http.PostForm(NETATMO_URL + "/oauth2/token", url.Values{
    "grant_type":    {"refresh_token"},
    "refresh_token": {os.Getenv("NETATMO_REFRESH_TOKEN")},
    "client_id":     {os.Getenv("NETATMO_CLIENT_ID")},
    "client_secret": {os.Getenv("NETATMO_CLIENT_SECRET")},
  })
	if err != nil {
		log.Fatal(err)
	}

  defer respToken.Body.Close()

	body, err := io.ReadAll(respToken.Body)
	if err != nil {
		log.Fatal(err)
	}

  var tokenData NetatmoApiToken
  json.Unmarshal(body, &tokenData)

  if tokenData.Error != nil {
    log.Fatal(tokenData.Error)
  }

  req, err := http.NewRequest("GET", NETATMO_URL + "/api/getstationsdata?device_id=" + os.Getenv("NETATMO_DEVICE_ID"), nil)
  req.Header.Add("Authorization", "Bearer " + tokenData.AccessToken)

  client := &http.Client{}
  resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

  defer resp.Body.Close()

  body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

  var apiResponse NetatmoApiResponse
  json.Unmarshal(body, &apiResponse)

  if apiResponse.Error != nil {
    log.Fatal(apiResponse.Error)
  }

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_DEFAULT_REGION")),
	})

	if err != nil {
		log.Fatal(err)
	}

	svc := dynamodb.New(sess)

  uuidV4, err := uuid.NewRandom()
  if err != nil {
    log.Fatal(err)
  }

  dashboardData := apiResponse.Body.Devices[0].DashboardData
  item := DynamoDBNetatmo{
    uuidV4.String(),
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
    TableName: aws.String("weather_netatmo"),
  }
  _, err = svc.PutItem(input)
  if err != nil {
    log.Fatal(err)
  }
}
