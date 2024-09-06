package worker

import (
	"ddb-writer/internal/config"
	"ddb-writer/internal/dynamodb"
	"ddb-writer/internal/generator"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func StartWorkers(wg *sync.WaitGroup, client *dynamodb.Client, cfg *config.Config, recordChan <-chan generator.UserBehaviorRecord) {
	var recordsWritten int64
	var recordsWrittenLastSecond int64
	startTime := time.Now()

	numWorkers := 10
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.BatchWriteUserBehaviorRecords(cfg.TableName, recordChan, &recordsWritten, &recordsWrittenLastSecond)
		}()
	}

	go startTicker(startTime, &recordsWritten, &recordsWrittenLastSecond)
}

func startTicker(startTime time.Time, recordsWritten, recordsWrittenLastSecond *int64) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		elapsedSeconds := time.Since(startTime).Seconds()
		avgRecordsPerSecond := float64(atomic.LoadInt64(recordsWritten)) / elapsedSeconds
		currentTime := time.Now().Format("2006-01-02 15:04:05")
		recordsLastSecond := atomic.SwapInt64(recordsWrittenLastSecond, 0)
		retryCount := atomic.LoadInt64(&dynamodb.RetryCount)
		failureCount := atomic.LoadInt64(&dynamodb.FailureCount)

		fmt.Printf("[%s] Average records written per second: %.2f, Records written in last second: %d, Retry count: %d, Failure count: %d\n",
			currentTime, avgRecordsPerSecond, recordsLastSecond, retryCount, failureCount)
	}
}
