import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import {aws_dynamodb} from "aws-cdk-lib";

export class DdbUserBehaviorStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const ads_log_user = new dynamodb.Table(this, 'ads_log_user', {
      partitionKey: {name: 'user_id', type: aws_dynamodb.AttributeType.NUMBER},
      sortKey: {name: 'client_ts', type: aws_dynamodb.AttributeType.NUMBER},
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: cdk.RemovalPolicy.DESTROY
    })
  }
}
