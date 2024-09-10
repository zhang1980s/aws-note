package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

var client *cloudwatchlogs.Client

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	client = cloudwatchlogs.NewFromConfig(cfg)
}

func listLogGroups(ctx context.Context) ([]string, error) {
	var logGroupNames []string
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(client, &cloudwatchlogs.DescribeLogGroupsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, logGroup := range page.LogGroups {
			logGroupNames = append(logGroupNames, *logGroup.LogGroupName)
		}
	}

	return logGroupNames, nil
}

func createExportTask(ctx context.Context, logGroupName, destinationBucket, destinationPrefix string, startTime, endTime int64) error {
	input := &cloudwatchlogs.CreateExportTaskInput{
		LogGroupName:      &logGroupName,
		From:              &startTime,
		To:                &endTime,
		Destination:       &destinationBucket,
		DestinationPrefix: &destinationPrefix,
	}

	var err error
	backoff := time.Second

	for attempt := 0; attempt < 5; attempt++ {
		_, err = client.CreateExportTask(ctx, input)
		if err == nil {
			return nil
		}

		log.Printf("Attempt %d: Failed to create export task for %s: %v", attempt+1, logGroupName, err)
		time.Sleep(backoff)
		backoff *= 2
	}

	return fmt.Errorf("failed to create export task for %s after retries: %w", logGroupName, err)
}

func handler(ctx context.Context) error {
	destinationBucket := os.Getenv("DESTINATION_BUCKET")
	if destinationBucket == "" {
		return fmt.Errorf("DESTINATION_BUCKET environment variable not set")
	}

	endTime := time.Now().UTC().Truncate(24 * time.Hour)
	startTime := endTime.Add(-24 * time.Hour)

	startTimeMs := startTime.UnixNano() / int64(time.Millisecond)
	endTimeMs := endTime.UnixNano() / int64(time.Millisecond)

	logGroups, err := listLogGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list log groups: %w", err)
	}

	ticker := time.NewTicker(1000 * time.Millisecond) // 1 calls per second
	defer ticker.Stop()

	for _, logGroupName := range logGroups {
		<-ticker.C // Rate limit

		destinationPrefix := fmt.Sprintf("exportedlogs/%s/year=%d/month=%02d/day=%02d",
			logGroupName[1:],
			startTime.Year(), startTime.Month(), startTime.Day())

		err := createExportTask(ctx, logGroupName, destinationBucket, destinationPrefix, startTimeMs, endTimeMs)
		if err != nil {
			log.Printf("Failed to create export task for %s: %v", logGroupName, err)
		} else {
			log.Printf("Export task created for %s", logGroupName)
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
