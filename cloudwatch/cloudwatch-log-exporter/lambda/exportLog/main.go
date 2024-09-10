package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

var client *cloudwatchlogs.Client
var logger *log.Logger
var errorMessages []string

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	client = cloudwatchlogs.NewFromConfig(cfg)
	logger = setupLogging()
}

func setupLogging() *log.Logger {
	return log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
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

	backoff := time.Second

	for attempt := 0; attempt < 5; attempt++ {
		_, err := client.CreateExportTask(ctx, input)
		if err == nil {
			return nil
		}

		errorMessage := fmt.Sprintf("Attempt %d: Failed to create export task for %s: %v", attempt+1, logGroupName, err)
		errorMessages = append(errorMessages, errorMessage)
		logger.Printf("Error: %s", errorMessage)
		time.Sleep(backoff)
		backoff *= 40
	}

	return fmt.Errorf("failed to create export task for %s after retries", logGroupName)
}

func getExportTimeRange() (int64, int64) {
	endTime := time.Now().UTC().Truncate(24 * time.Hour)
	startTime := endTime.Add(-24 * time.Hour)

	if envEndTime := os.Getenv("EXPORT_END_TIME"); envEndTime != "" {
		if parsedTime, err := time.Parse(time.RFC3339, envEndTime); err == nil {
			endTime = parsedTime
		}
	}

	if envStartTime := os.Getenv("EXPORT_START_TIME"); envStartTime != "" {
		if parsedTime, err := time.Parse(time.RFC3339, envStartTime); err == nil {
			startTime = parsedTime
		}
	}

	return startTime.UnixNano() / int64(time.Millisecond), endTime.UnixNano() / int64(time.Millisecond)
}

func handler(ctx context.Context) error {
	destinationBucket := os.Getenv("DESTINATION_BUCKET")
	if destinationBucket == "" {
		return fmt.Errorf("DESTINATION_BUCKET environment variable not set")
	}

	startTimeMs, endTimeMs := getExportTimeRange()

	logGroups, err := listLogGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list log groups: %w", err)
	}

	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	for _, logGroupName := range logGroups {
		<-ticker.C

		startTime := time.Unix(0, startTimeMs*int64(time.Millisecond)).UTC()
		destinationPrefix := fmt.Sprintf("exportedlogs/%s/year=%d/month=%02d/day=%02d",
			logGroupName[1:],
			startTime.Year(), startTime.Month(), startTime.Day())

		logger.Printf("Creating export task for %s", logGroupName)
		logger.Printf("Export time range: %s to %s",
			time.Unix(0, startTimeMs*int64(time.Millisecond)).Format(time.RFC3339),
			time.Unix(0, endTimeMs*int64(time.Millisecond)).Format(time.RFC3339))

		err := createExportTask(ctx, logGroupName, destinationBucket, destinationPrefix, startTimeMs, endTimeMs)
		if err != nil {
			logger.Printf("Failed to create export task for %s: %v", logGroupName, err)
		} else {
			logger.Printf("Export task created for %s", logGroupName)
		}
	}

	if len(errorMessages) > 0 {
		errorJSON, _ := json.Marshal(errorMessages)
		logger.Printf("Errors encountered during export: %s", string(errorJSON))
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
