package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"ddb-writer/internal/app"
	"ddb-writer/internal/dao"
)

func main() {
	tableName, totalRecords, recordsPerMillisecond := parseFlags()
	svc, err := dao.InitializeDynamoDBClient()
	if err != nil {
		log.Fatalf("unable to initialize DynamoDB client, %v", err)
	}

	recordChan := make(chan int, *totalRecords)
	var wg sync.WaitGroup
	var recordsWritten int64
	var recordsWrittenLastSecond int64
	startTime := time.Now()
	numWorkers := 100

	app.StartWorkers(&wg, svc, tableName, recordChan, &recordsWritten, &recordsWrittenLastSecond, numWorkers)
	app.StartTicker(startTime, &recordsWritten, &recordsWrittenLastSecond)

	generateRecords(totalRecords, recordsPerMillisecond, recordChan)
	close(recordChan)
	wg.Wait()

	printSummary(totalRecords, startTime)
}

func parseFlags() (*string, *int, *int) {
	tableName := flag.String("t", "trade", "DynamoDB table name")
	totalRecords := flag.Int("n", 100000, "Total number of records to write")
	recordsPerMillisecond := flag.Int("r", 1, "Number of records to write per millisecond")
	flag.Parse()
	return tableName, totalRecords, recordsPerMillisecond
}

func generateRecords(totalRecords, recordsPerMillisecond *int, recordChan chan<- int) {
	for i := 0; i < *totalRecords; i++ {
		recordChan <- i + 1
		if (i+1)%*recordsPerMillisecond == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func printSummary(totalRecords *int, startTime time.Time) {
	elapsedSeconds := time.Since(startTime).Seconds()
	avgRecordsPerSecond := float64(*totalRecords) / elapsedSeconds
	fmt.Printf("Total records written: %d\n", *totalRecords)
	fmt.Printf("Total time: %.2f seconds\n", elapsedSeconds)
	fmt.Printf("Final average records written per second: %.2f\n", avgRecordsPerSecond)
}
