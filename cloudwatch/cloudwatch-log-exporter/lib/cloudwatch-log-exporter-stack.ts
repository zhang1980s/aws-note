import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';


export class CloudwatchLogExporterStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

const destinationBucket = new cdk.CfnParameter(this, 'S3DestinationBucket', {
      type: 'String',
      description: 'The bucket name of destination of CreateExportTask',
      noEcho: false,
      default: 'myBucket'
    });

    const scheduleParameter = new cdk.CfnParameter(this, 'ScheduleParameter', {
      type: 'String',
      description: 'The cron schedule for the event rule',
      noEcho: false,
      default: 'cron(5 0 * * ? *)'  // 00:05:00 UTC
    });


    const exportLogFunction = new lambda.Function(this, 'export-log-function', {
        runtime: lambda.Runtime.PROVIDED_AL2023,
        architecture: lambda.Architecture.ARM_64,
        handler: 'bootstrap',
        code: lambda.Code.fromAsset('lambda/exportLog'),
        timeout: cdk.Duration.minutes(15),
        environment: {
            DESTINATION_BUCKET: destinationBucket.valueAsString,
        },
    })

    const exportLogVersion = exportLogFunction.currentVersion;

    const exportLogAlias = new lambda.Alias(this, 'export-log-prod', {
      aliasName: 'Prod',
      version: exportLogVersion,
    });


    const cloudwatchlogPolicyStatement = new iam.PolicyStatement({
      actions: ['logs:DescribeLogGroups', 'logs:CreateExportTask'],
      resources: ['*'],
    });

    const s3PolicyStatement = new iam.PolicyStatement({
      actions: ['s3:PutObject'],
      resources: [`arn:aws:s3:::${destinationBucket.valueAsString}/exportedlogs/*`],
    });

    exportLogFunction.addToRolePolicy(cloudwatchlogPolicyStatement);
    exportLogFunction.addToRolePolicy(s3PolicyStatement);


    const rule = new events.Rule(this, 'DailyTriggerRule', {
      schedule: events.Schedule.expression(scheduleParameter.valueAsString),
    });


    rule.addTarget(new targets.LambdaFunction(exportLogAlias));
  }
}
