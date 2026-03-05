import {
  CfnOutput,
  RemovalPolicy,
  Stack,
  StackProps
} from "aws-cdk-lib";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as rds from "aws-cdk-lib/aws-rds";
import * as sns from "aws-cdk-lib/aws-sns";
import { Construct } from "constructs";

export interface FoundationStackProps extends StackProps {
  appName: string;
  envName: string;
  vpcId?: string;
}

export class FoundationStack extends Stack {
  readonly vpc: ec2.IVpc;
  readonly dbCluster: rds.DatabaseCluster;
  readonly dbSecurityGroup: ec2.SecurityGroup;
  readonly databaseName: string;
  readonly eventTopic: sns.Topic;

  constructor(scope: Construct, id: string, props: FoundationStackProps) {
    super(scope, id, props);

    this.vpc = props.vpcId
      ? ec2.Vpc.fromLookup(this, "Vpc", { vpcId: props.vpcId })
      : new ec2.Vpc(this, "Vpc", {
        maxAzs: 2,
        natGateways: 1
      });

    this.dbSecurityGroup = new ec2.SecurityGroup(this, "DbSg", {
      vpc: this.vpc,
      allowAllOutbound: true,
      description: "Aurora PostgreSQL security group"
    });

    this.databaseName = "accountlink";
    this.dbCluster = new rds.DatabaseCluster(this, "Database", {
      engine: rds.DatabaseClusterEngine.auroraPostgres({
        version: rds.AuroraPostgresEngineVersion.VER_16_11
      }),
      writer: rds.ClusterInstance.serverlessV2("writer"),
      serverlessV2MinCapacity: 0.5,
      serverlessV2MaxCapacity: 2,
      defaultDatabaseName: this.databaseName,
      credentials: rds.Credentials.fromGeneratedSecret("accountlink"),
      vpc: this.vpc,
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS
      },
      securityGroups: [this.dbSecurityGroup],
      removalPolicy: RemovalPolicy.DESTROY,
      deletionProtection: false
    });

    this.eventTopic = new sns.Topic(this, "EventTopic", {
      topicName: `${props.appName}-${props.envName}-events`
    });

    new CfnOutput(this, "AuroraEndpoint", {
      value: this.dbCluster.clusterEndpoint.hostname
    });
    new CfnOutput(this, "AuroraPort", {
      value: this.dbCluster.clusterEndpoint.port.toString()
    });
    new CfnOutput(this, "AuroraSecretArn", {
      value: this.dbCluster.secret?.secretArn ?? ""
    });
    new CfnOutput(this, "EventTopicArn", {
      value: this.eventTopic.topicArn
    });
  }
}
