package dao

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"log"
	"math/rand"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type DynamoDBClient struct {
	Client *dynamodb.Client
}

type dashboardRecord struct {
	KKey string   `dynamodbav:"kkey"`
	TS   int64    `dynamodbav:"ts"`
	C    float64  `dynamodbav:"c"`
	CT   *int     `dynamodbav:"ct,omitempty"`
	H    float64  `dynamodbav:"h"`
	L    float64  `dynamodbav:"l"`
	O    float64  `dynamodbav:"o"`
	Q    *float64 `dynamodbav:"q,omitempty"`
	V    *float64 `dynamodbav:"v,omitempty"`
	N    *float64 `dynamodbav:"n,omitempty"`
}

type tradeRecord struct {
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

var tidCounter int64 = 1000000

func InitializeDynamoDBClient() (*DynamoDBClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}
	return &DynamoDBClient{Client: dynamodb.NewFromConfig(cfg)}, nil
}

func WriteDashboardRecords(svc *DynamoDBClient, tableName *string, recordChan <-chan int, recordsWritten *int64, recordsWrittenLastSecond *int64) {

	for recordID := range recordChan {
		record := createDashboardRecord()
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			log.Printf("failed to marshal record %d, %v", recordID, err)
			continue
		}

		_, err = svc.Client.PutItem(context.TODO(), &dynamodb.PutItemInput{
			TableName: tableName,
			Item:      item,
		})
		if err != nil {
			log.Printf("failed to put item %d, %v", recordID, err)
			continue
		}

		atomic.AddInt64(recordsWritten, 1)
		atomic.AddInt64(recordsWrittenLastSecond, 1)
	}
}

func WriteTradeRecords(svc *DynamoDBClient, tableName *string, recordChan <-chan int, recordsWritten *int64, recordsWrittenLastSecond *int64) {

	for recordID := range recordChan {
		record := createTradeRecord()
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			log.Printf("failed to marshal record %d, %v", recordID, err)
			continue
		}

		_, err = svc.Client.PutItem(context.TODO(), &dynamodb.PutItemInput{
			TableName: tableName,
			Item:      item,
		})
		if err != nil {
			log.Printf("failed to put item %d, %v", recordID, err)
			continue
		}

		atomic.AddInt64(recordsWritten, 1)
		atomic.AddInt64(recordsWrittenLastSecond, 1)
	}
}

func BatchWriteTradeRecords(svc *DynamoDBClient, tableName *string, recordChan <-chan int, recordsWritten *int64, recordsWrittenLastSecond *int64) {

	const batchSize = 25
	var batch []types.WriteRequest

	for recordID := range recordChan {
		record := createTradeRecord()
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			log.Printf("failed to marshal record %d, %v", recordID, err)
			continue
		}

		batch = append(batch, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})

		if len(batch) == batchSize {
			err := writeBatch(svc, tableName, batch)

			if err != nil {
				log.Printf("failed to write batch, %v", err)
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

	response, err := svc.Client.BatchWriteItem(context.TODO(), input)

	if err != nil {
		return fmt.Errorf("failed to write batch, %w", err)
	}

	if len(response.UnprocessedItems) > 0 {
		input.RequestItems = response.UnprocessedItems

		for len(response.UnprocessedItems) > 0 {
			response, err = svc.Client.BatchWriteItem(context.TODO(), input)
			if err != nil {
				return fmt.Errorf("failed to retry unprocessed items, %w", err)
			}
		}
	}
	return nil
}

func createDashboardRecord() dashboardRecord {
	record := dashboardRecord{
		KKey: fmt.Sprintf("%d_%d_%d_%d", int(randomFloat(0, 10000)), int(randomFloat(0, 100)), int(randomFloat(0, 9)), int(randomFloat(0, 1000000))),
		TS:   time.Now().UnixNano() / int64(time.Millisecond),
		C:    randomFloat(60000, 70000),
		H:    randomFloat(60000, 70000),
		L:    randomFloat(60000, 70000),
		O:    randomFloat(60000, 70000),
	}

	if rand.Float32() < 0.5 {
		ct := rand.Intn(500000)
		record.CT = &ct
	}
	if rand.Float32() < 0.5 {
		q := randomFloat(0, 10)
		record.Q = &q
	}
	if rand.Float32() < 0.5 {
		v := randomFloat(0, 100)
		record.N = &v
	}
	if rand.Float32() < 0.5 {
		n := randomFloat(0, 100)
		record.N = &n
	}

	//fmt.Printf("Created dashboardRecord: %+v\n", record)
	return record
}

func createTradeRecord() tradeRecord {
	cids := []int{11111, 22222, 33333, 44444, 55555, 66666, 77777, 88888, 99999}
	//weights := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.6, 0.1}
	weights := []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0}
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
	}

	record := tradeRecord{
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

	//fmt.Printf("Created tradeRecord: %+v\n", record)
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
	value := min + rand.Float64()*(max-min)
	valueStr := fmt.Sprintf("%.5f", value)
	value, _ = strconv.ParseFloat(valueStr, 64)
	return value
}
