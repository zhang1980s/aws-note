import * as cdk from 'aws-cdk-lib';
import {RemovalPolicy} from 'aws-cdk-lib';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import {AttributeType} from 'aws-cdk-lib/aws-dynamodb';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as sfn from 'aws-cdk-lib/aws-stepfunctions';
import * as tasks from 'aws-cdk-lib/aws-stepfunctions-tasks';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';
import {Construct} from 'constructs';
import * as path from 'path';

export class CloudwatchLogExporterStepFunctionsStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const destinationBucket = new cdk.CfnParameter(this, 'S3DestinationBucket', {
        type: 'String',
        description: 'The bucket name of destination of CreateExportTask',
        noEcho: false,
        default: 'myBucket'
    });

    const logPrefix = new cdk.CfnParameter(this, 'logPrefix', {
        type: 'String',
        description: 'The logPrefix in destination bucket',
        noEcho: false,
        default: 'myPrefix'
    });

    const scheduleParameter = new cdk.CfnParameter(this, 'ScheduleParameter', {
          type: 'String',
          description: 'The cron schedule for the event rule',
          noEcho: false,
          default: 'cron(5 0 * * ? *)'  // UTC
    });

    const table = new dynamodb.Table(this, 'ExportTasksTable', {
        partitionKey: { name: 'Region', type: dynamodb.AttributeType.STRING },
        sortKey: { name: 'Name', type: AttributeType.STRING},
        billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
        removalPolicy: RemovalPolicy.DESTROY,
    });

    const exportLambda = new lambda.Function(this, 'ExportLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2023,
        architecture: lambda.Architecture.ARM_64,
        handler: 'bootstrap',
        code: lambda.Code.fromAsset(path.join(__dirname, '../lambda'), {
            bundling: {
                image: lambda.Runtime.PROVIDED_AL2023.bundlingImage,
                command: [
                    'bash', '-c', [
                        'export GOARCH=arm64 GOOS=linux',
                        'export GOPATH=/tmp/go',
                        'mkdir -p /tmp/go',
                        'go build -tags lambda.norpc -o bootstrap main.go',
                        'cp bootstrap /asset-output/'
                    ].join(' && ')
                ],
                user: 'root',
            },
        }),
        environment: {
            DYNAMODB_TABLE_NAME: table.tableName,
        },
    });

    table.grantReadWriteData(exportLambda);
    exportLambda.addToRolePolicy(new iam.PolicyStatement({
        actions: ['logs:DescribeLogGroups', 'logs:CreateExportTask', 'logs:DescribeExportTasks'],
        resources: ['*'],
    }));

    const listLogGroups = new tasks.LambdaInvoke(this, 'ListLogGroups', {
          lambdaFunction: exportLambda,
          payload: sfn.TaskInput.fromObject({action: 'listLogGroups'}),
      });

    const checkRunningTasks = new tasks.LambdaInvoke(this, 'CheckRunningTasks', {
        lambdaFunction: exportLambda,
        payload: sfn.TaskInput.fromObject({ action: 'checkRunningTasks' }),
        outputPath: '$.Payload'
    });


    const getNextLogGroup = new tasks.LambdaInvoke(this, 'GetNextLogGroup', {
        lambdaFunction: exportLambda,
        payload: sfn.TaskInput.fromObject({ action: 'getNextLogGroup' }),
        resultPath: '$.logGroupResult',
    });

    const createExportTask = new tasks.LambdaInvoke(this, 'CreateExportTask', {
        lambdaFunction: exportLambda,
        payloadResponseOnly: true,
        payload: sfn.TaskInput.fromObject({
            action: 'createExportTask',
            logGroupName: sfn.JsonPath.stringAt('$.logGroupResult.Payload.Name'),
            s3BucketName: destinationBucket.valueAsString,
            s3Prefix: logPrefix.valueAsString,
            region: sfn.JsonPath.stringAt('$.logGroupResult.Payload.Region'),
        }),
        resultPath: '$.TaskId',
    });

    const updateDynamoDB = new tasks.LambdaInvoke(this, 'UpdateDynamoDB', {
        lambdaFunction: exportLambda,
        payloadResponseOnly: true,
        payload: sfn.TaskInput.fromObject({
            action: 'updateDynamoDB',
            logGroupName: sfn.JsonPath.stringAt('$.logGroupResult.Payload.Name'),
            status: 'COMPLETED',
            taskId: sfn.JsonPath.stringAt('$.TaskId.taskId'),
        }),
    });

    const wait = new sfn.Wait(this,'Wait',{
        time:sfn.WaitTime.duration(cdk.Duration.seconds(30)),
    });


    const definition = listLogGroups
        .next(checkRunningTasks)
        .next(new sfn.Choice(this, 'AreTasksRunning')
            .when(sfn.Condition.booleanEquals('$.tasksRunning', true), wait.next(checkRunningTasks))
            .otherwise(getNextLogGroup
                .next(new sfn.Choice(this, 'LogGroupAvailable')
                    .when(sfn.Condition.isNotNull('$.logGroupResult.Payload.Name'),
                        createExportTask
                            .next(updateDynamoDB)
                            .next(checkRunningTasks))
                    .otherwise(new sfn.Succeed(this, 'AllLogGroupsProcessed'))
                )
            )
        );

    const stateMachine = new sfn.StateMachine(this,'ExportStateMachine',{
        definitionBody:sfn.DefinitionBody.fromChainable(definition),
        timeout :cdk.Duration.hours(24),
    });


    new events.Rule(this, 'DailyExportRule', {
        schedule: events.Schedule.expression(scheduleParameter.valueAsString),
        targets: [new targets.SfnStateMachine(stateMachine)],
    });


    exportLambda.addToRolePolicy(new iam.PolicyStatement({
        actions: ['s3:PutObject'],
        resources: ['arn:aws:s3:::${destinationBucket.valueAsString}/${logPrefix.valueAsString}/*'],
    }));
  }
}

