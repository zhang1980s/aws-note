package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var (
	dynamoClient *dynamodb.Client
	cwLogsClient *cloudwatchlogs.Client
	ssmClient    *ssm.Client
	snsClient    *sns.Client
	tableName    string
	ssmParamName string
	exportDays   int
	snsTopic     string
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg)
	cwLogsClient = cloudwatchlogs.NewFromConfig(cfg)
	ssmClient = ssm.NewFromConfig(cfg)
	snsClient = sns.NewFromConfig(cfg)

	tableName = os.Getenv("DYNAMODB_TABLE_NAME")
	ssmParamName = os.Getenv("SSM_PARAM_NAME")
	exportDays, _ = strconv.Atoi(os.Getenv("EXPORT_DAYS"))
	if exportDays == 0 {
		exportDays = 1
	}
	snsTopic = os.Getenv("SNS_TOPIC_ARN")
}

func main() {
	lambda.Start(HandleRequest)
}
