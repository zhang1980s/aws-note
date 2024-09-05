import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import {aws_dynamodb} from "aws-cdk-lib";
import * as lambda from 'aws-cdk-lib/aws-lambda'
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';

export class DdbStreamHotKeyStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const streamReader new lambda.Function(this, 'stream-reader', {
      runtime: lambda.Runtime.PROVIDED_AL2023,
      architecture: lambda.Architecture.ARM_64,
      handler: 'bootstrap',
      code: lambda.Code.fromAsset('code/lambda/stream-writer'),
      timeout: cdk.Duration.seconds(10),
      environment:
    })
    const tradeTable = new dynamodb.Table(this, 'trade', {
      partitionKey: {name: 'tid', type: aws_dynamodb.AttributeType.NUMBER},
      sortKey: {name: 'cid', type: aws_dynamodb.AttributeType.NUMBER},
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST
    })

    const readTradeTable = new dynamodb.Table(this,'read-trade', {
      partitionKey: { name: 'cid', type: aws_dynamodb.AttributeType.NUMBER},
      sortKey: {name: 'tid', type: aws_dynamodb.AttributeType.NUMBER},
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST
    })



  }
}
