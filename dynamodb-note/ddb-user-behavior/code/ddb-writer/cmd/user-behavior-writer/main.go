package main

import (
	"ddb-writer/internal/config"
	"ddb-writer/internal/dynamodb"
	"ddb-writer/internal/generator"
	"ddb-writer/internal/worker"
	"log"
	"sync"
)

func main() {
	cfg := config.Load()
	client, err := dynamodb.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize DynamoDB client: %v", err)
	}

	recordChan := make(chan generator.UserBehaviorRecord, cfg.RecordsPerSecond*2)
	var wg sync.WaitGroup

	worker.StartWorkers(&wg, client, cfg, recordChan)
	generator.GenerateUserBehaviorRecords(cfg.TotalRecords, cfg.RecordsPerSecond, recordChan)

	close(recordChan)
	wg.Wait()
}
