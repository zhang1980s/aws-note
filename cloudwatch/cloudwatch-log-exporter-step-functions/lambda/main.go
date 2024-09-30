package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Event struct {
	Action       string `json:"action"`
	LogGroupName string `json:"logGroupName,omitempty"`
	S3BucketName string `json:"s3BucketName,omitempty"`
	S3Prefix     string `json:"s3Prefix,omitempty"`
	Status       string `json:"status,omitempty"`
	TaskId       string `json:"taskId,omitempty"`
}

type LogGroup struct {
	Region          string    `json:"region"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	TaskId          string    `json:"taskId,omitempty"`
	ExportStartTime time.Time `json:"exportStartTime,omitempty"`
	ExportEndTime   time.Time `json:"exportEndTime,omitempty"`
}

var (
	dynamoClient  *dynamodb.Client
	cwLogsClient  *cloudwatchlogs.Client
	tableName     string
	currentRegion string
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg)
	cwLogsClient = cloudwatchlogs.NewFromConfig(cfg)
	tableName = os.Getenv("DYNAMODB_TABLE_NAME")
	currentRegion = cfg.Region
}

func handler(ctx context.Context, event Event) (interface{}, error) {
	log.Printf("Received event: %+v", event)

	switch event.Action {
	case "listLogGroups":
		err := listLogGroups(ctx)
		return map[string]interface{}{"success": err == nil}, err
	case "checkRunningTasks":
		tasksRunning, err := checkRunningTasks(ctx)
		return map[string]interface{}{"tasksRunning": tasksRunning}, err
	case "getNextLogGroup":
		return getNextLogGroup(ctx)
	case "createExportTask":
		taskId, err := createExportTask(ctx, event)
		return map[string]interface{}{"taskId": taskId}, err
	case "updateDynamoDB":
		err := updateDynamoDB(ctx, event)
		return map[string]interface{}{"success": err == nil}, err
	default:
		return nil, fmt.Errorf("unknown action: %s", event.Action)
	}
}

func listLogGroups(ctx context.Context) error {
	log.Printf("Listing log groups in region: %s", currentRegion)
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(cwLogsClient, &cloudwatchlogs.DescribeLogGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Printf("Error listing log groups: %v", err)
			return err
		}
		for _, logGroup := range page.LogGroups {
			log.Printf("Found log group: %s", *logGroup.LogGroupName)
			_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: &tableName,
				Item: map[string]dynamodbtypes.AttributeValue{
					"Region": &dynamodbtypes.AttributeValueMemberS{Value: currentRegion},
					"Name":   &dynamodbtypes.AttributeValueMemberS{Value: *logGroup.LogGroupName},
					"Status": &dynamodbtypes.AttributeValueMemberS{Value: "PENDING"},
				},
			})
			if err != nil {
				log.Printf("Error writing log group to DynamoDB: %v", err)
				return err
			}
		}
	}
	return nil
}

func checkRunningTasks(ctx context.Context) (map[string]interface{}, error) {
	log.Printf("Checking for running or pending export tasks in region: %s", currentRegion)

	input := &cloudwatchlogs.DescribeExportTasksInput{
		StatusCode: types.ExportTaskStatusCodePending,
	}

	output, err := cwLogsClient.DescribeExportTasks(ctx, input)
	if err != nil {
		log.Printf("Error checking pending tasks: %v", err)
		return map[string]interface{}{"tasksRunning": false}, err
	}

	if len(output.ExportTasks) > 0 {
		log.Printf("Pending tasks found")
		return map[string]interface{}{"tasksRunning": true}, nil
	}

	input.StatusCode = types.ExportTaskStatusCodeRunning
	output, err = cwLogsClient.DescribeExportTasks(ctx, input)
	if err != nil {
		log.Printf("Error checking running tasks: %v", err)
		return map[string]interface{}{"tasksRunning": false}, err
	}

	tasksRunning := len(output.ExportTasks) > 0
	log.Printf("Running tasks found: %t", tasksRunning)
	return map[string]interface{}{"tasksRunning": tasksRunning}, nil
}

func getNextLogGroup(ctx context.Context) (map[string]interface{}, error) {
	log.Println("Getting next pending log group...")
	input := &dynamodb.QueryInput{
		TableName:              &tableName,
		KeyConditionExpression: aws.String("#region = :region"),
		FilterExpression:       aws.String("#status = :pending"),
		ExpressionAttributeNames: map[string]string{
			"#status": "Status",
			"#region": "Region",
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":pending": &dynamodbtypes.AttributeValueMemberS{Value: "PENDING"},
			":region":  &dynamodbtypes.AttributeValueMemberS{Value: currentRegion},
		},
		Limit: aws.Int32(1),
	}
	output, err := dynamoClient.Query(ctx, input)
	if err != nil {
		log.Printf("Error querying DynamoDB for next log group: %v", err)
		return nil, err
	}
	if len(output.Items) == 0 {
		log.Println("No pending log groups found.")
		return map[string]interface{}{
			"Name":   nil,
			"Region": nil,
		}, nil
	}
	var logGroup LogGroup
	err = attributevalue.UnmarshalMap(output.Items[0], &logGroup)
	if err != nil {
		log.Printf("Error unmarshalling DynamoDB item: %v", err)
		return nil, err
	}
	return map[string]interface{}{
		"Name":   logGroup.Name,
		"Region": logGroup.Region,
	}, nil
}

func createExportTask(ctx context.Context, event Event) (map[string]interface{}, error) {
	log.Printf("Creating export task for log group: %s in region: %s", event.LogGroupName, currentRegion)

	now := time.Now()
	from := now.Add(-24 * time.Hour)

	createTaskInput := &cloudwatchlogs.CreateExportTaskInput{
		Destination:       aws.String(event.S3BucketName),
		DestinationPrefix: aws.String(event.S3Prefix),
		From:              aws.Int64(from.UnixNano() / 1000000),
		LogGroupName:      aws.String(event.LogGroupName),
		To:                aws.Int64(now.UnixNano() / 1000000),
	}

	output, err := cwLogsClient.CreateExportTask(ctx, createTaskInput)
	if err != nil {
		log.Printf("Error creating export task: %v", err)
		return nil, err
	}

	log.Printf("Export task created with ID: %s", *output.TaskId)

	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"Region": &dynamodbtypes.AttributeValueMemberS{Value: currentRegion},
			"Name":   &dynamodbtypes.AttributeValueMemberS{Value: event.LogGroupName},
		},
		UpdateExpression: aws.String("SET #status = :running, #taskId = :taskId, #startTime = :startTime"),
		ExpressionAttributeNames: map[string]string{
			"#status":    "Status",
			"#taskId":    "TaskId",
			"#startTime": "ExportStartTime",
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":running":   &dynamodbtypes.AttributeValueMemberS{Value: "RUNNING"},
			":taskId":    &dynamodbtypes.AttributeValueMemberS{Value: *output.TaskId},
			":startTime": &dynamodbtypes.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		},
	}

	_, err = dynamoClient.UpdateItem(ctx, updateInput)
	if err != nil {
		log.Printf("Error updating DynamoDB with export start time and task ID")
		return nil, fmt.Errorf("failed to update DynamoDB with export start time and task ID")
	}

	return map[string]interface{}{
		"taskId": *output.TaskId,
	}, nil
}

func updateDynamoDB(ctx context.Context, event Event) error {
	log.Printf("Updating DynamoDB for log group: %s", event.LogGroupName)

	now := time.Now()

	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"Region": &dynamodbtypes.AttributeValueMemberS{Value: currentRegion},
			"Name":   &dynamodbtypes.AttributeValueMemberS{Value: event.LogGroupName},
		},
		UpdateExpression: aws.String("SET #status = :status, #endTime = :endTime"),
		ExpressionAttributeNames: map[string]string{
			"#status":  "Status",
			"#endTime": "ExportEndTime",
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":status":  &dynamodbtypes.AttributeValueMemberS{Value: event.Status},
			":endTime": &dynamodbtypes.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		},
	}

	_, err := dynamoClient.UpdateItem(ctx, updateInput)
	if err != nil {
		log.Printf("Error updating DynamoDB with export end time and status completed")
		return fmt.Errorf("failed to update DynamoDB with export end time and status completed")
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
