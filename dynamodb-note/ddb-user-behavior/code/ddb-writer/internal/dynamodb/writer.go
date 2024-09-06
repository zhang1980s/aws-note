package dynamodb

import (
	"context"
	"ddb-writer/internal/generator"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"log"
	"sync/atomic"
)

var (
	RetryCount   int64
	FailureCount int64
)

func (c *Client) BatchWriteUserBehaviorRecords(tableName string, records <-chan generator.UserBehaviorRecord, recordsWritten, recordsWrittenLastSecond *int64) {
	const maxBatchSize = 25
	var batch []types.WriteRequest

	for record := range records {
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			log.Printf("failed to marshal record %d, %v", record.USER_ID, err)
			atomic.AddInt64(&FailureCount, 1)
			continue
		}

		batch = append(batch, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})

		if len(batch) == maxBatchSize {
			if err := c.writeBatch(tableName, batch); err != nil {
				log.Printf("failed to write batch: %v", err)
				atomic.AddInt64(&FailureCount, int64(len(batch)))
			} else {
				atomic.AddInt64(recordsWritten, int64(len(batch)))
				atomic.AddInt64(recordsWrittenLastSecond, int64(len(batch)))
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := c.writeBatch(tableName, batch); err != nil {
			log.Printf("failed to write final batch: %v", err)
			atomic.AddInt64(&FailureCount, int64(len(batch)))
		} else {
			atomic.AddInt64(recordsWritten, int64(len(batch)))
			atomic.AddInt64(recordsWrittenLastSecond, int64(len(batch)))
		}
	}
}

func (c *Client) writeBatch(tableName string, batch []types.WriteRequest) error {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: batch,
		},
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		response, err := c.BatchWriteItem(context.TODO(), input)
		if err != nil {
			atomic.AddInt64(&RetryCount, 1)
			if i == maxRetries-1 {
				return fmt.Errorf("failed to write batch after %d retries: %w", maxRetries, err)
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
