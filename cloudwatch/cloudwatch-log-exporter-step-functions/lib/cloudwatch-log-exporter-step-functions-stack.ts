import * as cdk from 'aws-cdk-lib';
import { RemovalPolicy } from 'aws-cdk-lib';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import { AttributeType } from 'aws-cdk-lib/aws-dynamodb';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as sfn from 'aws-cdk-lib/aws-stepfunctions';
import * as tasks from 'aws-cdk-lib/aws-stepfunctions-tasks';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';
import * as ssm from 'aws-cdk-lib/aws-ssm';
import * as sns from 'aws-cdk-lib/aws-sns';
import { Construct } from 'constructs';
import * as path from 'path';

export class CloudwatchLogExporterStepFunctionsStack extends cdk.Stack {
    constructor(scope: Construct, id: string, props?: cdk.StackProps) {
        super(scope, id, props);

        // Create SSM Parameter
        const regionBucketParam = new ssm.StringParameter(this, 'RegionBucketParam', {
            parameterName: '/cloudwatch-log-exporter/region-bucket-map',
            stringValue: 'us-east-1,s3://aaa\nus-east-2,s3://bbb',
            description: 'Mapping of regions to S3 buckets for CloudWatch log export',
        });

        // Create SNS Topic for failed exports
        const failedExportsTopic = new sns.Topic(this, 'FailedExportsTopic', {
            topicName: 'cloudwatch-log-export-failures',
        });

        // Create CloudFormation parameters
        const logPrefix = new cdk.CfnParameter(this, 'logPrefix', {
            type: 'String',
            description: 'The logPrefix in destination bucket',
            default: 'myPrefix',
        });

        const scheduleParameter = new cdk.CfnParameter(this, 'ScheduleParameter', {
            type: 'String',
            description: 'The cron schedule for the event rule',
            default: 'cron(5 0 * * ? *)', // UTC
        });

        const exportDaysParameter = new cdk.CfnParameter(this, 'ExportDaysParameter', {
            type: 'Number',
            description: 'Number of days of logs to export',
            default: 1,
            minValue: 1,
        });

        // Create DynamoDB table
        const table = new dynamodb.Table(this, 'ExportTasksTable', {
            partitionKey: { name: 'Region', type: dynamodb.AttributeType.STRING },
            sortKey: { name: 'Name', type: AttributeType.STRING },
            billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
            removalPolicy: RemovalPolicy.DESTROY,
        });

        table.addGlobalSecondaryIndex({
            indexName: 'ItemStatusIndex',
            partitionKey: { name: 'ItemStatus', type: dynamodb.AttributeType.STRING },
        });

        // Create Lambda function
        const exportLambda = new lambda.Function(this, 'ExportLambda', {
            runtime: lambda.Runtime.PROVIDED_AL2023,
            architecture: lambda.Architecture.ARM_64,
            handler: 'bootstrap',
            timeout: cdk.Duration.minutes(10),
            code: lambda.Code.fromAsset(path.join(__dirname, '../lambda'), {
                bundling: {
                    image: lambda.Runtime.PROVIDED_AL2023.bundlingImage,
                    command: [
                        'bash',
                        '-c',
                        [
                            'export GOARCH=arm64 GOOS=linux',
                            'export GOPATH=/tmp/go',
                            'mkdir -p /tmp/go',
                            'go build -tags lambda.norpc -o bootstrap',
                            'cp bootstrap /asset-output/'
                        ].join(' && ')
                    ],
                    user: 'root',
                },
            }),
            environment: {
                DYNAMODB_TABLE_NAME: table.tableName,
                SSM_PARAM_NAME: regionBucketParam.parameterName,
                EXPORT_DAYS: exportDaysParameter.valueAsString,
                SNS_TOPIC_ARN: failedExportsTopic.topicArn,
            },
        });

        // Grant permissions to Lambda
        table.grantReadWriteData(exportLambda);
        regionBucketParam.grantRead(exportLambda);
        failedExportsTopic.grantPublish(exportLambda);
        exportLambda.addToRolePolicy(new iam.PolicyStatement({
            actions: [
                'logs:DescribeLogGroups',
                'logs:CreateExportTask',
                'logs:DescribeExportTasks',
                'logs:ListTagsForResource',
            ],
            resources: ['*'],
        }));
        exportLambda.addToRolePolicy(new iam.PolicyStatement({
            actions: ['s3:PutObject'],
            resources: ['arn:aws:s3:::*/*'],
        }));

        // Define Step Functions tasks
        const sendNotification = new tasks.LambdaInvoke(this, 'SendNotification', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({
                action: 'sendNotification',
                topicArn: failedExportsTopic.topicArn,
                message: sfn.JsonPath.stringAt('$.error'),
            }),
        });

        const listLogGroups = new tasks.LambdaInvoke(this, 'ListLogGroups', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({ action: 'listLogGroups' }),
        }).addCatch(sendNotification, {
            resultPath: '$.error',
        });

        const getNextLogGroup = new tasks.LambdaInvoke(this, 'GetNextLogGroup', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({ action: 'getNextLogGroup' }),
            resultPath: '$.logGroupResult',
        }).addCatch(sendNotification, {
            resultPath: '$.error',
        });

        const checkRunningTasks = new tasks.LambdaInvoke(this, 'CheckRunningTasks', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({
                action: 'checkRunningTasks',
                region: sfn.JsonPath.stringAt('$.logGroupResult.Payload.region'),
            }),
            resultPath: '$.checkTasksResult',
        }).addCatch(sendNotification, {
            resultPath: '$.error',
        });

        const createExportTask = new tasks.LambdaInvoke(this, 'CreateExportTask', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({
                action: 'createExportTask',
                logGroupName: sfn.JsonPath.stringAt('$.logGroupResult.Payload.name'),
                region: sfn.JsonPath.stringAt('$.logGroupResult.Payload.region'),
            }),
            resultPath: '$.createTaskResult',
        }).addCatch(sendNotification, {
            resultPath: '$.error',
        });

        const checkExportTaskStatus = new tasks.LambdaInvoke(this, 'CheckExportTaskStatus', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({
                action: 'checkExportTaskStatus',
                taskId: sfn.JsonPath.stringAt('$.createTaskResult.Payload.taskId'),
                region: sfn.JsonPath.stringAt('$.logGroupResult.Payload.region'),
            }),
            resultPath: '$.checkStatusResult',
        }).addCatch(sendNotification, {
            resultPath: '$.error',
        });

        const notifyFailure = new tasks.LambdaInvoke(this, 'NotifyFailure', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({
                action: 'sendNotification',
                topicArn: failedExportsTopic.topicArn,
                message: sfn.JsonPath.format('Export task failed for log group {} in region {}. Task ID: {}, Status: {}, Start Time: {}',
                    sfn.JsonPath.stringAt('$.logGroupResult.Payload.name'),
                    sfn.JsonPath.stringAt('$.logGroupResult.Payload.region'),
                    sfn.JsonPath.stringAt('$.createTaskResult.Payload.taskId'),
                    sfn.JsonPath.stringAt('$.checkStatusResult.Payload.status'),
                    sfn.JsonPath.stringAt('$.checkStatusResult.Payload.startTime')
                ),
            }),
        });

        const updateDynamoDB = new tasks.LambdaInvoke(this, 'UpdateDynamoDB', {
            lambdaFunction: exportLambda,
            payload: sfn.TaskInput.fromObject({
                action: 'updateDynamoDB',
                logGroupName: sfn.JsonPath.stringAt('$.logGroupResult.Payload.name'),
                region: sfn.JsonPath.stringAt('$.logGroupResult.Payload.region'),
                itemStatus: sfn.JsonPath.stringAt('$.checkStatusResult.Payload.status'),
                taskId: sfn.JsonPath.stringAt('$.createTaskResult.Payload.taskId'),
                startTime: sfn.JsonPath.stringAt('$.checkStatusResult.Payload.startTime'),
                endTime: sfn.JsonPath.stringAt('$.checkStatusResult.Payload.endTime'),
            }),
        }).addCatch(sendNotification, {
            resultPath: '$.error',
        });



        // Define wait states
        const wait30SecondsForTasks = new sfn.Wait(this, 'Wait30SecondsForTasks', {
            time: sfn.WaitTime.duration(cdk.Duration.seconds(3)),
        });

        const wait30SecondsForExport = new sfn.Wait(this, 'Wait30SecondsForExport', {
            time: sfn.WaitTime.duration(cdk.Duration.seconds(3)),
        });

        const wait30SecondsForStatus = new sfn.Wait(this, 'Wait30SecondsForStatus', {
            time: sfn.WaitTime.duration(cdk.Duration.seconds(3)),
        });

        // Define Step Functions workflow
        const definition = listLogGroups
            .next(getNextLogGroup)
            .next(new sfn.Choice(this, 'LogGroupAvailable')
                .when(sfn.Condition.isPresent('$.logGroupResult.Payload.name'),
                    checkRunningTasks
                        .next(new sfn.Choice(this, 'AreTasksRunning')
                            .when(sfn.Condition.booleanEquals('$.checkTasksResult.Payload.tasksRunning', true),
                                wait30SecondsForTasks.next(checkRunningTasks))
                            .otherwise(
                                createExportTask
                                    .next(wait30SecondsForExport)
                                    .next(checkExportTaskStatus)
                                    .next(new sfn.Choice(this, 'ExportTaskStatus')
                                        .when(sfn.Condition.stringEquals('$.checkStatusResult.Payload.status.Code', 'COMPLETED'),
                                            updateDynamoDB)
                                        .when(sfn.Condition.or(
                                            sfn.Condition.stringEquals('$.checkStatusResult.Payload.status.Code', 'CANCELLED'),
                                            sfn.Condition.stringEquals('$.checkStatusResult.Payload.status.Code', 'FAILED'),
                                            sfn.Condition.stringEquals('$.checkStatusResult.Payload.status.Code', 'PENDING_CANCEL')
                                        ), notifyFailure)
                                        .otherwise(wait30SecondsForStatus.next(checkExportTaskStatus))
                                    )
                            )
                        )
                )
                .otherwise(new sfn.Succeed(this, 'AllLogGroupsProcessed'))
            );

        notifyFailure.next(updateDynamoDB);
        updateDynamoDB.next(getNextLogGroup);

        // Create Step Functions state machine
        const stateMachine = new sfn.StateMachine(this, 'ExportStateMachine', {
            definitionBody: sfn.DefinitionBody.fromChainable(definition),
            timeout: cdk.Duration.hours(24),
        });

        // Create EventBridge rule to trigger the state machine
        new events.Rule(this, 'DailyExportRule', {
            schedule: events.Schedule.expression(scheduleParameter.valueAsString),
            targets: [new targets.SfnStateMachine(stateMachine)],
        });
    }
}