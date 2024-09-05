package dao

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBClient struct {
	Client *dynamodb.Client
}

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

var (
	tidCounter   int64 = 1000000
	RetryCount   int64
	FailureCount int64
)

func InitializeDynamoDBClient() (*DynamoDBClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}
	return &DynamoDBClient{Client: dynamodb.NewFromConfig(cfg)}, nil
}

func BatchWriteTradeRecords(svc *DynamoDBClient, tableName *string, recordChan <-chan TradeRecord, recordsWritten *int64, recordsWrittenLastSecond *int64) {
	const maxBatchItem = 25
	var batch []types.WriteRequest

	for record := range recordChan {
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			log.Printf("failed to marshal record %d, %v", record.TID, err)
			atomic.AddInt64(&FailureCount, 1)
			continue
		}

		batch = append(batch, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})

		if len(batch) == maxBatchItem {
			err := writeBatch(svc, tableName, batch)
			if err != nil {
				log.Printf("failed to write batch, %v", err)
				atomic.AddInt64(&FailureCount, int64(len(batch)))
			} else {
				atomic.AddInt64(recordsWritten, int64(len(batch)))
				atomic.AddInt64(recordsWrittenLastSecond, int64(len(batch)))
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		err := writeBatch(svc, tableName, batch)
		if err != nil {
			log.Printf("failed to write final batch, %v", err)
			atomic.AddInt64(&FailureCount, int64(len(batch)))
		} else {
			atomic.AddInt64(recordsWritten, int64(len(batch)))
			atomic.AddInt64(recordsWrittenLastSecond, int64(len(batch)))
		}
	}
}

func writeBatch(svc *DynamoDBClient, tableName *string, batch []types.WriteRequest) error {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			*tableName: batch,
		},
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		response, err := svc.Client.BatchWriteItem(context.TODO(), input)
		if err != nil {
			atomic.AddInt64(&RetryCount, 1)
			if i == maxRetries-1 {
				return fmt.Errorf("failed to write batch after %d retries, %w", maxRetries, err)
			}
			continue
		}

		if len(response.UnprocessedItems) == 0 {
			return nil
		}

		input.RequestItems = response.UnprocessedItems
		atomic.AddInt64(&RetryCount, 1)
	}

	return fmt.Errorf("failed to write all items after %d retries", maxRetries)
}

func CreateTradeRecord() TradeRecord {
	cids := []int{11111, 22222, 33333, 44444, 55555, 66666, 77777, 88888, 99999, 00000}
	weights := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}
	cid := weightedRandomChoice(cids, weights)
	tid := int(atomic.AddInt64(&tidCounter, 1))

	idMap := map[int]string{
		11111: "AAAAA-BBBBB",
		22222: "CCCCC-DDDDD",
		33333: "EEEEE-FFFFF",
		44444: "GGGGG-HHHHH",
		55555: "IIIII-JJJJJ",
		66666: "KKKKK-LLLLL",
		77777: "MMMMM-NNNNN",
		88888: "OOOOO-PPPPP",
		99999: "QQQQQ-RRRRR",
		00000: "SSSSS-TTTTT",
	}

	record := TradeRecord{
		TID: tid,
		CID: cid,
		B:   1,
		IID: idMap[cid],
		MID: cid % 10000,
		PX:  randomFloat(0.0000, 1.0000),
		S:   randomChoice([]string{"buy", "sell"}),
		SZ:  randomFloat(0.000001, 99999.99999),
		TS:  time.Now().UnixNano() / int64(time.Millisecond),
	}

	return record
}

func weightedRandomChoice(choices []int, weights []float64) int {
	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}

	randValue := rand.Float64() * totalWeight
	for i, weight := range weights {
		if randValue < weight {
			return choices[i]
		}
		randValue -= weight
	}

	return choices[len(choices)-1]
}

func randomChoice(choices []string) string {
	return choices[rand.Intn(len(choices))]
}

func randomFloat(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}
