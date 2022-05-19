package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type foodieBuckRequest struct {
	PK        string `json:"pk"`
	SK        string `json:"sk"`
	TableName string `json:"tableName"`
}

// FoodieBuckProfile should have all pk USER# sk PROFILE#
// that would be expected to return from the DDB Table
// aws-sdk-go-v2 requires the struct to use dynamodbav type
type FoodieBuckProfile struct {
	PK        string `dynamodbav:"pk"`
	SK        string `dynamodbav:"sk"`
	Name      string `dynamodbav:"displayName"`
	Available int    `dynamodbav:"foodieBucksAvailable"`
	Used      int    `dynamodbav:"foodieBucksUsed"`
	Increment int    `dynamodbav:"foodieBuckIncrement"`
}

var tableName = "dev.glennmeyer.dev-foodiebucks"

var errorLogger = log.New(os.Stderr, "ERROR ", log.Llongfile)

// Helper function to convert API Body to JSON Request
func unmarshalAPIRequest(r events.APIGatewayV2HTTPRequest) (foodieBuckRequest, error) {
	fbr := foodieBuckRequest{}
	err := json.Unmarshal([]byte(r.Body), &fbr)
	return fbr, err
}

// Helper function to return custom error response
func clientError(sc int) (events.APIGatewayV2HTTPResponse, error) {
	return events.APIGatewayV2HTTPResponse{
		StatusCode: sc,
		Body:       http.StatusText(sc),
	}, nil
}

// Helpers for error handling; logs to os.Stderr
func serverError(err error) (events.APIGatewayV2HTTPResponse, error) {
	errorLogger.Println(err.Error())

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       http.StatusText(http.StatusInternalServerError),
	}, nil
}

// We can only pass one func to Lambda, so we switch on METHOD
func router(r events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	fbr, err := unmarshalAPIRequest(r)
	if err != nil {
		clientError(http.StatusUnprocessableEntity)
	}
	switch r.RequestContext.HTTP.Method {
	case "GET":
		return getProfile(fbr)
	default:
		return clientError(http.StatusMethodNotAllowed)
	}
}

// GetProfile queries DDB and returns marshalled response
func getProfile(r foodieBuckRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-2"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Using the Config value, create the DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	// Build the request with its input parameters
	data, err := svc.GetItem(context.TODO(), &dynamodb.GetItemInput{
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: r.PK},
			"sk": &types.AttributeValueMemberS{Value: r.SK},
		},
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Fatalf("failed to get item, %v", err)
	}

	// Write response data to AttributeValue map and unmarshal
	p := FoodieBuckProfile{}
	err = attributevalue.UnmarshalMap(data.Item, &p)
	if err != nil {
		log.Printf("Couldn't unmarshal response. Here's why: %v\n", err)
	}

	// Marshal profile back into JSON
	json, err := json.Marshal(p)
	if err != nil {
		return serverError(err)
	}

	// Return response data as proper type
	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       string(json),
	}, nil
}

func main() {
	lambda.Start(router)
}
