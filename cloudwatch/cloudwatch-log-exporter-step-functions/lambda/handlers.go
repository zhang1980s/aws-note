package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Event struct {
	Action       string `json:"action"`
	LogGroupName string `json:"logGroupName,omitempty"`
	Region       string `json:"region,omitempty"`
	Status       string `json:"status,omitempty"`
	TaskId       string `json:"taskId,omitempty"`
	StartTime    int64  `json:"startTime,omitempty"`
	EndTime      int64  `json:"endTime,omitempty"`
	TopicArn     string `json:"topicArn,omitempty"`
	Message      string `json:"message,omitempty"`
}

type LogGroup struct {
	Region     string    `json:"region"`
	Name       string    `json:"name"`
	ItemStatus string    `json:"itemStatus"`
	TaskId     string    `json:"taskId,omitempty"`
	StartTime  time.Time `json:"startTime,omitempty"`
	EndTime    time.Time `json:"endTime,omitempty"`
}

type RegionBucketMap struct {
	Region string
	Bucket string
}

func HandleRequest(ctx context.Context, event Event) (interface{}, error) {
	log.Printf("Received event: %+v", event)

	switch event.Action {
	case "listLogGroups":
		return listLogGroups(ctx)
	case "checkRunningTasks":
		return checkRunningTasks(ctx, event.Region)
	case "getNextLogGroup":
		return getNextLogGroup(ctx)
	case "createExportTask":
		return createExportTask(ctx, event)
	case "checkExportTaskStatus":
		return checkExportTaskStatus(ctx, event)
	case "updateDynamoDB":
		return updateDynamoDB(ctx, event)
	case "notifyFailure":
		return notifyFailure(ctx, event)
	case "sendNotification":
		return sendNotification(ctx, event)
	default:
		return nil, fmt.Errorf("unknown action: %s", event.Action)
	}
}

func getRegionBucketMap(ctx context.Context) ([]RegionBucketMap, error) {
	param, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(ssmParamName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get SSM parameter: %v", err)
	}

	var regionBucketMap []RegionBucketMap
	for _, line := range strings.Split(*param.Parameter.Value, "\n") {
		parts := strings.Split(line, ",")
		if len(parts) == 2 {
			regionBucketMap = append(regionBucketMap, RegionBucketMap{
				Region: strings.TrimSpace(parts[0]),
				Bucket: strings.TrimSpace(parts[1]),
			})
		}
	}
	return regionBucketMap, nil
}

func getAccountID(ctx context.Context) string {
	lambdaContext, ok := lambdacontext.FromContext(ctx)
	if !ok {
		log.Println("Could not retrieve Lambda context")
		return ""
	}
	arnParts := strings.Split(lambdaContext.InvokedFunctionArn, ":")
	if len(arnParts) >= 5 {
		return arnParts[4]
	}
	return ""
}

func listLogGroups(ctx context.Context) (interface{}, error) {
	regionBucketMap, err := getRegionBucketMap(ctx)
	if err != nil {
		return nil, err
	}

	for _, rbm := range regionBucketMap {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(rbm.Region))
		if err != nil {
			log.Printf("Error loading config for region %s: %v", rbm.Region, err)
			continue
		}

		cwLogsClient := cloudwatchlogs.NewFromConfig(cfg)
		paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(cwLogsClient, &cloudwatchlogs.DescribeLogGroupsInput{})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				log.Printf("Error listing log groups in region %s: %v", rbm.Region, err)
				continue
			}

			for _, logGroup := range page.LogGroups {
				logGroupArn := fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", rbm.Region, getAccountID(ctx), aws.ToString(logGroup.LogGroupName))

				tags, err := cwLogsClient.ListTagsForResource(ctx, &cloudwatchlogs.ListTagsForResourceInput{
					ResourceArn: aws.String(logGroupArn),
				})
				if err != nil {
					log.Printf("Error listing tags for log group %s: %v", aws.ToString(logGroup.LogGroupName), err)
					// Continue processing even if tag listing fails
					tags = &cloudwatchlogs.ListTagsForResourceOutput{} // Empty tags
				}

				if value, exists := tags.Tags["auto-backup"]; exists && value == "no" {
					continue
				}

				// Add log group to DynamoDB
				_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
					TableName: aws.String(tableName),
					Item: map[string]dynamodbtypes.AttributeValue{
						"Region":     &dynamodbtypes.AttributeValueMemberS{Value: rbm.Region},
						"Name":       &dynamodbtypes.AttributeValueMemberS{Value: aws.ToString(logGroup.LogGroupName)},
						"ItemStatus": &dynamodbtypes.AttributeValueMemberS{Value: "PENDING"},
					},
				})
				if err != nil {
					log.Printf("Error writing log group to DynamoDB: %v", err)
				}
			}
		}
	}

	return map[string]bool{"success": true}, nil
}

// FIXME: check all regions listed in parameter store
func checkRunningTasks(ctx context.Context, region string) (interface{}, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("error loading config for region %s: %v", region, err)
	}

	cwLogsClient := cloudwatchlogs.NewFromConfig(cfg)
	input := &cloudwatchlogs.DescribeExportTasksInput{}
	output, err := cwLogsClient.DescribeExportTasks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error describing export tasks: %v", err)
	}

	for _, task := range output.ExportTasks {
		if task.Status != nil && (task.Status.Code == types.ExportTaskStatusCodeRunning || task.Status.Code == types.ExportTaskStatusCodePending) {
			return map[string]bool{"tasksRunning": true}, nil
		}
	}

	return map[string]bool{"tasksRunning": false}, nil
}

func getNextLogGroup(ctx context.Context) (interface{}, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ItemStatusIndex"), // Add a GSI for ItemStatus
		KeyConditionExpression: aws.String("ItemStatus = :status"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":status": &dynamodbtypes.AttributeValueMemberS{Value: "PENDING"},
		},
		Limit: aws.Int32(1),
	}

	output, err := dynamoClient.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error querying DynamoDB: %v", err)
	}

	if len(output.Items) == 0 {
		return nil, nil
	}

	var logGroup LogGroup
	err = attributevalue.UnmarshalMap(output.Items[0], &logGroup)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling DynamoDB item: %v", err)
	}

	return logGroup, nil
}

func createExportTask(ctx context.Context, event Event) (interface{}, error) {
	log.Printf("Starting createExportTask for log group: %s in region: %s", event.LogGroupName, event.Region)

	regionBucketMap, err := getRegionBucketMap(ctx)
	if err != nil {
		log.Printf("Error getting region bucket map: %v", err)
		return nil, fmt.Errorf("failed to get region bucket map: %v", err)
	}
	log.Printf("Retrieved region bucket map: %+v", regionBucketMap)

	var destinationBucket string
	for _, rbm := range regionBucketMap {
		if rbm.Region == event.Region {
			destinationBucket = rbm.Bucket
			break
		}
	}

	if destinationBucket == "" {
		log.Printf("No destination bucket found for region: %s", event.Region)
		return nil, fmt.Errorf("no destination bucket found for region %s", event.Region)
	}
	log.Printf("Destination bucket for region %s: %s", event.Region, destinationBucket)

	// Extract bucket name from S3 URI
	bucketName := strings.TrimPrefix(destinationBucket, "s3://")
	log.Printf("Extracted bucket name: %s", bucketName)

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(event.Region))
	if err != nil {
		log.Printf("Error loading AWS config for region %s: %v", event.Region, err)
		return nil, fmt.Errorf("error loading config for region %s: %v", event.Region, err)
	}

	cwLogsClient := cloudwatchlogs.NewFromConfig(cfg)

	now := time.Now().UTC()
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -exportDays)
	log.Printf("Exporting logs from %s to %s", from.Format(time.RFC3339), now.Format(time.RFC3339))

	destinationPrefix := fmt.Sprintf("%s/%s", event.LogGroupName, now.Format("2006/01/02"))
	log.Printf("Destination prefix: %s", destinationPrefix)

	input := &cloudwatchlogs.CreateExportTaskInput{
		Destination:       aws.String(bucketName),
		LogGroupName:      aws.String(event.LogGroupName),
		From:              aws.Int64(from.UnixNano() / 1000000),
		To:                aws.Int64(now.UnixNano() / 1000000),
		DestinationPrefix: aws.String(destinationPrefix),
	}

	log.Printf("CreateExportTask input: Destination=%s, LogGroupName=%s, From=%s, To=%s, DestinationPrefix=%s",
		*input.Destination,
		*input.LogGroupName,
		time.Unix(0, *input.From*int64(time.Millisecond)).Format(time.RFC3339),
		time.Unix(0, *input.To*int64(time.Millisecond)).Format(time.RFC3339),
		*input.DestinationPrefix)

	output, err := cwLogsClient.CreateExportTask(ctx, input)
	if err != nil {
		log.Printf("Error creating export task: %v", err)
		return nil, fmt.Errorf("error creating export task: %v", err)
	}

	log.Printf("Export task created successfully. Task ID: %s", *output.TaskId)

	return map[string]string{"taskId": *output.TaskId}, nil
}

func checkExportTaskStatus(ctx context.Context, event Event) (interface{}, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(event.Region))
	if err != nil {
		return nil, fmt.Errorf("error loading config for region %s: %v", event.Region, err)
	}

	cwLogsClient := cloudwatchlogs.NewFromConfig(cfg)
	input := &cloudwatchlogs.DescribeExportTasksInput{
		TaskId: aws.String(event.TaskId),
	}

	output, err := cwLogsClient.DescribeExportTasks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error describing export task: %v", err)
	}

	if len(output.ExportTasks) == 0 {
		return nil, fmt.Errorf("export task not found")
	}

	task := output.ExportTasks[0]
	return map[string]interface{}{
		"status":    task.Status,
		"startTime": task.From,
		"endTime":   task.To,
	}, nil
}

func updateDynamoDB(ctx context.Context, event Event) (interface{}, error) {
	startTime := time.Unix(0, event.StartTime*int64(time.Millisecond))
	endTime := time.Unix(0, event.EndTime*int64(time.Millisecond))

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"Region": &dynamodbtypes.AttributeValueMemberS{Value: event.Region},
			"Name":   &dynamodbtypes.AttributeValueMemberS{Value: event.LogGroupName},
		},
		UpdateExpression: aws.String("SET #itemStatus = :itemstatus, #taskId = :taskid, #startTime = :starttime, #endTime = :endtime"),
		ExpressionAttributeNames: map[string]string{
			"#itemStatus": "ItemStatus",
			"#taskId":     "TaskId",
			"#startTime":  "StartTime",
			"#endTime":    "EndTime",
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":itemstatus": &dynamodbtypes.AttributeValueMemberS{Value: event.Status},
			":taskid":     &dynamodbtypes.AttributeValueMemberS{Value: event.TaskId},
			":starttime":  &dynamodbtypes.AttributeValueMemberS{Value: startTime.Format(time.RFC3339)},
			":endtime":    &dynamodbtypes.AttributeValueMemberS{Value: endTime.Format(time.RFC3339)},
		},
	}

	_, err := dynamoClient.UpdateItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error updating DynamoDB: %w", err)
	}

	return map[string]bool{"success": true}, nil
}

func notifyFailure(ctx context.Context, event Event) (interface{}, error) {
	startTime := time.Unix(0, event.StartTime*int64(time.Millisecond))
	message := fmt.Sprintf("Export task failed for log group %s in region %s. Task ID: %s, Status: %s, Start Time: %s",
		event.LogGroupName, event.Region, event.TaskId, event.Status, startTime.Format(time.RFC3339))

	input := &sns.PublishInput{
		Message:  aws.String(message),
		TopicArn: aws.String(snsTopic),
	}

	_, err := snsClient.Publish(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error publishing to SNS: %v", err)
	}

	return map[string]bool{"success": true}, nil
}

func sendNotification(ctx context.Context, event Event) (interface{}, error) {
	input := &sns.PublishInput{
		Message:  aws.String(event.Message),
		TopicArn: aws.String(event.TopicArn),
	}

	_, err := snsClient.Publish(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error publishing to SNS: %v", err)
	}

	return map[string]bool{"success": true}, nil
}
