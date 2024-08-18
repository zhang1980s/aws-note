package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
	tableName, indexName, cidValue := parseFlags()

	// Load the Shared AWS Configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create a DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	// Execute the query
	queryDynamoDB(svc, tableName, indexName, cidValue)
}

func parseFlags() (string, string, int64) {
	tableName := flag.String("t", "trade", "DynamoDB table name")
	indexName := flag.String("i", "gsi-cid-tid", "DynamoDB index name")
	cidValue := flag.Int64("c", 88888, "CID value to query")
	flag.Parse()

	return *tableName, *indexName, *cidValue
}

func queryDynamoDB(svc *dynamodb.Client, tableName, indexName string, cidValue int64) {
	// Build the expression for the query
	keyCond := expression.Key("cid").Equal(expression.Value(cidValue))

	expr, err := expression.NewBuilder().WithKeyCondition(keyCond).Build()
	if err != nil {
		log.Fatalf("failed to build expression, %v", err)
	}

	// Query the GSI
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(tableName),
		IndexName:                 aws.String(indexName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	result, err := svc.Query(context.TODO(), input)
	if err != nil {
		log.Fatalf("failed to query items, %v", err)
	}

	timeIntervals := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second, 90 * time.Second, 120 * time.Second}

	// Initialize maps to hold statistics for each interval
	counts := make(map[time.Duration]int32)
	buyCounts := make(map[time.Duration]int32)
	sellCounts := make(map[time.Duration]int32)
	szSums := make(map[time.Duration]float64)
	szCounts := make(map[time.Duration]int32)

	// Current time in milliseconds
	currentTime := time.Now().UnixNano() / int64(time.Millisecond)

	for _, item := range result.Items {
		var s string
		var ts int64
		var sz float64

		// Unmarshal attributes
		if err := attributevalue.Unmarshal(item["s"], &s); err != nil {
			log.Printf("failed to unmarshal 's', %v", err)
			continue
		}
		if err := attributevalue.Unmarshal(item["ts"], &ts); err != nil {
			log.Printf("failed to unmarshal 'ts', %v", err)
			continue
		}
		if err := attributevalue.Unmarshal(item["sz"], &sz); err != nil {
			log.Printf("failed to unmarshal 'sz', %v", err)
			continue
		}

		// Calculate statistics for each time interval
		for _, interval := range timeIntervals {
			if currentTime-ts <= int64(interval.Milliseconds()) {
				counts[interval]++
				if s == "buy" {
					buyCounts[interval]++
				} else if s == "sell" {
					sellCounts[interval]++
				}
				szSums[interval] += sz
				szCounts[interval]++
			}
		}
	}

	// Print results for each interval
	for _, interval := range timeIntervals {
		averageSz := 0.0
		if szCounts[interval] > 0 {
			averageSz = szSums[interval] / float64(szCounts[interval])
		}
		currentTimestamp := time.Now().Format(time.RFC3339)
		fmt.Printf("[%s] Interval: %s, Count: %d, Buy Count: %d, Sell Count: %d, Average SZ: %.2f\n",
			currentTimestamp, interval, counts[interval], buyCounts[interval], sellCounts[interval], averageSz)
	}
}
