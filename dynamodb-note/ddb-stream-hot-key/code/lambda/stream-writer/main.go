package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"log"
)

// Define the structures for the records
type TradeRecord struct {
	TID int     `dynamodbav:"tid"`
	CID int     `dynamodbav:"cid"`
	B   int     `dynamodbav:"b"`
	IID string  `dynamodbav:"iid"`
	MID int     `dynamodbav:"mid"`
	PX  float64 `dynamodbav:"px"`
	S   string  `dynamodbav:"s"`
	SZ  float64 `dynamodbav:"sz"`
	TS  int64   `dynamodbav:"ts"`
}

type ActualTradeRecord struct {
	CID int     `dynamodbav:"cid"`
	TID int     `dynamodbav:"tid"`
	B   int     `dynamodbav:"b"`
	IID string  `dynamodbav:"iid"`
	MID int     `dynamodbav:"mid"`
	PX  float64 `dynamodbav:"px"`
	S   string  `dynamodbav:"s"`
	SZ  float64 `dynamodbav:"sz"`
	TS  int64   `dynamodbav:"ts"`
}

var svc *dynamodb.Client

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	svc = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	for _, record := range event.Records {
		if record.EventName == "INSERT" {
			// Convert the event's NewImage to a map of types.AttributeValue
			avMap := make(map[string]types.AttributeValue)
			for k, v := range record.Change.NewImage {
				avMap[k] = convertToAttributeValue(v)
			}

			var tradeRecord TradeRecord
			err := attributevalue.UnmarshalMap(avMap, &tradeRecord)
			if err != nil {
				log.Printf("Failed to unmarshal record: %v", err)
				continue
			}

			actualTradeRecord := ActualTradeRecord{
				CID: tradeRecord.CID,
				TID: tradeRecord.TID,
				B:   tradeRecord.B,
				IID: tradeRecord.IID,
				MID: tradeRecord.MID,
				PX:  tradeRecord.PX,
				S:   tradeRecord.S,
				SZ:  tradeRecord.SZ,
				TS:  tradeRecord.TS,
			}

			item, err := attributevalue.MarshalMap(actualTradeRecord)
			if err != nil {
				log.Printf("Failed to marshal actualTradeRecord: %v", err)
				continue
			}

			_, err = svc.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: aws.String("tableB"), // Replace with your actual table name
				Item:      item,
			})
			if err != nil {
				log.Printf("Failed to put item in tableB: %v", err)
			} else {
				fmt.Printf("Successfully wrote record to tableB: %v\n", actualTradeRecord)
			}
		}
	}
	return nil
}

// Helper function to convert events.DynamoDBAttributeValue to types.AttributeValue
func convertToAttributeValue(av events.DynamoDBAttributeValue) types.AttributeValue {
	switch {
	case av.Binary() != nil:
		return &types.AttributeValueMemberB{Value: av.Binary()}
	case av.Boolean():
		return &types.AttributeValueMemberBOOL{Value: av.Boolean()}
	case len(av.List()) > 0:
		list := make([]types.AttributeValue, len(av.List()))
		for i, v := range av.List() {
			list[i] = convertToAttributeValue(v)
		}
		return &types.AttributeValueMemberL{Value: list}
	case len(av.Map()) > 0:
		m := make(map[string]types.AttributeValue)
		for k, v := range av.Map() {
			m[k] = convertToAttributeValue(v)
		}
		return &types.AttributeValueMemberM{Value: m}
	case av.Number() != "":
		return &types.AttributeValueMemberN{Value: av.Number()}
	case av.String() != "":
		return &types.AttributeValueMemberS{Value: av.String()}
	case len(av.StringSet()) > 0:
		return &types.AttributeValueMemberSS{Value: av.StringSet()}
	case len(av.NumberSet()) > 0:
		return &types.AttributeValueMemberNS{Value: av.NumberSet()}
	case len(av.BinarySet()) > 0:
		return &types.AttributeValueMemberBS{Value: av.BinarySet()}
	default:
		return &types.AttributeValueMemberNULL{Value: true}
	}
}

func main() {
	lambda.Start(handler)
}
