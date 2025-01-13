package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type Record struct {
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

func main() {

	tableName := flag.String("t", "DynamodbCdcStack-sourcetable70CF4744-1T9IBS4KQDN1W", "DynamoDB table name")
	totalRecords := flag.Int("n", 100000, "Total number of records to write")
	recordsPerMillisecond := flag.Int("r", 1, "Number of records to write per millisecond")
	flag.Parse()

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	svc := dynamodb.NewFromConfig(cfg)

	recordChan := make(chan int, *totalRecords)

	var wg sync.WaitGroup
	var recordsWritten int64

	startTime := time.Now()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writeRecords(svc, tableName, recordChan, &recordsWritten)
		}()
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			elapsedSeconds := time.Since(startTime).Seconds()
			avgRecordsPerSecond := float64(atomic.LoadInt64(&recordsWritten)) / elapsedSeconds
			fmt.Printf("Average records written per second: %.2f\n", avgRecordsPerSecond)
		}
	}()

	for i := 0; i < *totalRecords; i++ {
		recordChan <- i + 1
		if (i+1)%*recordsPerMillisecond == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	close(recordChan)
	wg.Wait()

	elapsedSeconds := time.Since(startTime).Seconds()
	avgRecordsPerSecond := float64(*totalRecords) / elapsedSeconds
	fmt.Printf("Total records written: %d\n", *totalRecords)
	fmt.Printf("Total time: %.2f seconds\n", elapsedSeconds)
	fmt.Printf("Final average records written per second: %.2f\n", avgRecordsPerSecond)
}

func writeRecords(svc *dynamodb.Client, tableName *string, recordChan <-chan int, recordsWritten *int64) {
	for recordID := range recordChan {
		//timestamp := time.Now().UnixNano() / int64(time.Millisecond)

		record := Record{
			//KKey: fmt.Sprintf("%d_%d_%d_%d", 10000, 17, 2, 86400),
			//TS:   timestamp,
			KKey: fmt.Sprintf("%d_%d_%d_%d", int(randomFloat(0, 10000)), int(randomFloat(0, 100)), int(randomFloat(0, 9)), int(randomFloat(0, 1000000))),
			TS:   1721491200000,
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
			v := randomFloat(1000000, 5000000)
			record.V = &v
		}
		if rand.Float32() < 0.5 {
			n := randomFloat(0, 100)
			record.N = &n
		}

		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			log.Printf("failed to marshal record %d, %v", recordID, err)
			continue
		}

		_, err = svc.PutItem(context.TODO(), &dynamodb.PutItemInput{
			TableName: tableName,
			Item:      item,
		})
		if err != nil {
			log.Printf("failed to put item %d, %v", recordID, err)
			continue
		}

		atomic.AddInt64(recordsWritten, 1)
	}
}

func randomFloat(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}
