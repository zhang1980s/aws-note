package main

import (
	"ddb-writer/internal/app"
	"ddb-writer/internal/dao"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"
)

func main() {
	tableName, totalRecords, recordsPerSecond := parseFlags()
	svc, err := dao.InitializeDynamoDBClient()
	if err != nil {
		log.Fatalf("unable to initialize DynamoDB client, %v", err)
	}

	recordChan := make(chan dao.TradeRecord, *recordsPerSecond*2)
	var wg sync.WaitGroup
	var recordsWritten int64
	var recordsWrittenLastSecond int64

	startTime := time.Now()
	numWorkers := 10

	app.StartWorkers(&wg, svc, tableName, recordChan, &recordsWritten, &recordsWrittenLastSecond, numWorkers)
	app.StartTicker(startTime, &recordsWritten, &recordsWrittenLastSecond)

	generateRecords(totalRecords, recordsPerSecond, recordChan)

	close(recordChan)
	wg.Wait()

	printSummary(totalRecords, startTime)
}

func parseFlags() (*string, *int, *int) {
	tableName := flag.String("t", "trade", "DynamoDB table name")
	totalRecords := flag.Int("n", 100000, "Total number of records to write")
	recordsPerSecond := flag.Int("r", 100, "Number of records to write per second (max 2000000)")
	flag.Parse()

	if *recordsPerSecond > 2000000 {
		log.Fatalf("Records per second cannot exceed 2000000")
	}

	return tableName, totalRecords, recordsPerSecond
}

func generateRecords(totalRecords, recordsPerSecond *int, recordChan chan<- dao.TradeRecord) {
	batchSize := *recordsPerSecond
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for i := 0; i < *totalRecords; i += batchSize {
		batch := make([]dao.TradeRecord, 0, batchSize)
		for j := 0; j < batchSize && i+j < *totalRecords; j++ {
			batch = append(batch, dao.CreateTradeRecord())
		}

		for _, record := range batch {
			recordChan <- record
		}

		<-ticker.C
	}
}

func printSummary(totalRecords *int, startTime time.Time) {
	elapsedSeconds := time.Since(startTime).Seconds()
	avgRecordsPerSecond := float64(*totalRecords) / elapsedSeconds
	fmt.Printf("Total records written: %d\n", *totalRecords)
	fmt.Printf("Total time: %.2f seconds\n", elapsedSeconds)
	fmt.Printf("Final average records written per second: %.2f\n", avgRecordsPerSecond)
	fmt.Printf("Total retry count: %d\n", dao.RetryCount)
	fmt.Printf("Total failure count: %d\n", dao.FailureCount)
}
