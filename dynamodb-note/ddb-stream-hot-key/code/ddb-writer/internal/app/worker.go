package app

import (
	"ddb-writer/internal/dao"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func StartWorkers(wg *sync.WaitGroup, svc *dao.DynamoDBClient, tableName *string, recordChan <-chan dao.TradeRecord, recordsWritten *int64, recordsWrittenLastSecond *int64, numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dao.BatchWriteTradeRecords(svc, tableName, recordChan, recordsWritten, recordsWrittenLastSecond)
		}()
	}
}

func StartTicker(startTime time.Time, recordsWritten *int64, recordsWrittenLastSecond *int64) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			elapsedSeconds := time.Since(startTime).Seconds()
			avgRecordsPerSecond := float64(atomic.LoadInt64(recordsWritten)) / elapsedSeconds
			currentTime := time.Now().Format("2006-01-02 15:04:05")
			recordsLastSecond := atomic.SwapInt64(recordsWrittenLastSecond, 0)
			retryCount := atomic.LoadInt64(&dao.RetryCount)
			failureCount := atomic.LoadInt64(&dao.FailureCount)
			fmt.Printf("[%s] Average records written per second: %.2f, Records written in last second: %d, Retry count: %d, Failure count: %d\n",
				currentTime, avgRecordsPerSecond, recordsLastSecond, retryCount, failureCount)
		}
	}()
}
