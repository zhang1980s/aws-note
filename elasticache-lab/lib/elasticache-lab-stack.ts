import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as elasticache from 'aws-cdk-lib/aws-elasticache';
import { Construct } from 'constructs';

export class ElasticacheLabStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // Use default VPC
    const vpc = ec2.Vpc.fromLookup(this, 'DefaultVPC', {
      isDefault: true
    });

    const parameterGroup = new elasticache.CfnParameterGroup(this, 'redis-mylab-7.x-cluster-ParameterGroup', {
      cacheParameterGroupFamily: 'redis7',  // for Redis 7.x
      description: 'Parameter group for Redis 7.x with LFU policy',
      properties:{
        'maxmemory-policy': 'allkeys-lfu',
        'cluster-enabled': 'yes' // Enable cluster mode
      },
    });

    // Create a security group for Redis
    const redisSecurityGroup = new ec2.SecurityGroup(this, 'redis-mylab-7.x-SecurityGroup', {
      vpc,
      description: 'Security group for Redis cluster',
      allowAllOutbound: true
    });

    // Allow inbound traffic on port 6379
    redisSecurityGroup.addIngressRule(
      ec2.Peer.anyIpv6(),
      ec2.Port.tcp(6379),
      'Allow Redis traffic'
    );

    // Create a subnet group
    const subnetGroup = new elasticache.CfnSubnetGroup(this, 'redis-mylab-7.x-SubnetGroup', {
      description: 'Subnet group for Redis cluster',
      subnetIds: vpc.publicSubnets.map(subnet => subnet.subnetId)
    });


    const replicationGroup = new elasticache.CfnReplicationGroup(this, 'redis-mylab-7.x-Cluster', {
      replicationGroupDescription: 'Redis lab cluster with cluster mode',
      engine: 'redis',
      engineVersion: '7.1',  // Specify Redis 7.x version
      cacheNodeType: 'cache.r7g.large',  // Adjust as needed
      numNodeGroups: 2,  // Number of shards for cluster mode
      replicasPerNodeGroup: 1,  // Number of replicas per shard
      cacheSubnetGroupName: subnetGroup.ref,
      autoMinorVersionUpgrade: true,
      automaticFailoverEnabled: true,
      cacheParameterGroupName: parameterGroup.ref,
      securityGroupIds: [redisSecurityGroup.securityGroupId],
      port: 6379,
      clusterMode: 'enabled',  // Enable cluster mode
  
    });
  }
}
