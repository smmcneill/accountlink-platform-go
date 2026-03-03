import {
  CfnOutput,
  Duration,
  RemovalPolicy,
  Stack,
  StackProps
} from "aws-cdk-lib";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as elbv2 from "aws-cdk-lib/aws-elasticloadbalancingv2";
import * as logs from "aws-cdk-lib/aws-logs";
import * as rds from "aws-cdk-lib/aws-rds";
import { Construct } from "constructs";

export interface PlatformStackProps extends StackProps {
  appName: string;
  envName: string;
  imageAssetPath: string;
  imageAssetDockerfile: string;
  containerPort: number;
  desiredCount: number;
  vpcId?: string;
}

export class PlatformStack extends Stack {
  constructor(scope: Construct, id: string, props: PlatformStackProps) {
    super(scope, id, props);

    const vpc = props.vpcId
      ? ec2.Vpc.fromLookup(this, "Vpc", { vpcId: props.vpcId })
      : new ec2.Vpc(this, "Vpc", {
        maxAzs: 2,
        natGateways: 1
      });

    const cluster = new ecs.Cluster(this, "Cluster", {
      vpc,
      clusterName: `${props.appName}-${props.envName}`
    });

    const taskDefinition = new ecs.FargateTaskDefinition(this, "TaskDef", {
      cpu: 512,
      memoryLimitMiB: 1024
    });

    const logGroup = new logs.LogGroup(this, "AppLogs", {
      logGroupName: `/ecs/${props.appName}-${props.envName}`,
      retention: logs.RetentionDays.ONE_MONTH
    });

    const container = taskDefinition.addContainer("App", {
      image: ecs.ContainerImage.fromAsset(props.imageAssetPath, {
        file: props.imageAssetDockerfile
      }),
      logging: ecs.LogDrivers.awsLogs({
        streamPrefix: props.appName,
        logGroup
      }),
      portMappings: [
        {
          containerPort: props.containerPort
        }
      ]
    });

    const albSecurityGroup = new ec2.SecurityGroup(this, "AlbSg", {
      vpc,
      allowAllOutbound: true,
      description: "ALB security group"
    });
    albSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80), "Allow HTTP");

    const serviceSecurityGroup = new ec2.SecurityGroup(this, "ServiceSg", {
      vpc,
      allowAllOutbound: true,
      description: "ECS service security group"
    });
    serviceSecurityGroup.addIngressRule(albSecurityGroup, ec2.Port.tcp(props.containerPort), "Allow ALB to app");

    const dbSecurityGroup = new ec2.SecurityGroup(this, "DbSg", {
      vpc,
      allowAllOutbound: true,
      description: "Aurora PostgreSQL security group"
    });
    dbSecurityGroup.addIngressRule(serviceSecurityGroup, ec2.Port.tcp(5432), "Allow ECS service to Postgres");

    const databaseName = "accountlink";

    const service = new ecs.FargateService(this, "Service", {
      cluster,
      taskDefinition,
      desiredCount: props.desiredCount,
      assignPublicIp: false,
      securityGroups: [serviceSecurityGroup],
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS
      }
    });

    const dbCluster = new rds.DatabaseCluster(this, "Database", {
      engine: rds.DatabaseClusterEngine.auroraPostgres({
        version: rds.AuroraPostgresEngineVersion.VER_16_11
      }),
      writer: rds.ClusterInstance.serverlessV2("writer"),
      serverlessV2MinCapacity: 0.5,
      serverlessV2MaxCapacity: 2,
      defaultDatabaseName: databaseName,
      credentials: rds.Credentials.fromGeneratedSecret("accountlink"),
      vpc,
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS
      },
      securityGroups: [dbSecurityGroup],
      removalPolicy: RemovalPolicy.DESTROY,
      deletionProtection: false
    });

    container.addEnvironment("DB_HOST", dbCluster.clusterEndpoint.hostname);
    container.addEnvironment("DB_PORT", dbCluster.clusterEndpoint.port.toString());
    container.addEnvironment("DB_NAME", databaseName);
    container.addEnvironment("DB_SSL_MODE", "require");

    if (dbCluster.secret) {
      dbCluster.secret.grantRead(taskDefinition.taskRole);
      if (taskDefinition.executionRole) {
        dbCluster.secret.grantRead(taskDefinition.executionRole);
      }
      container.addSecret("DB_USER", ecs.Secret.fromSecretsManager(dbCluster.secret, "username"));
      container.addSecret("DB_PASSWORD", ecs.Secret.fromSecretsManager(dbCluster.secret, "password"));
    }

    const loadBalancer = new elbv2.ApplicationLoadBalancer(this, "Alb", {
      vpc,
      internetFacing: true,
      securityGroup: albSecurityGroup
    });

    const listener = loadBalancer.addListener("HttpListener", {
      port: 80,
      open: true
    });

    listener.addTargets("EcsTarget", {
      port: props.containerPort,
      protocol: elbv2.ApplicationProtocol.HTTP,
      targets: [service],
      healthCheck: {
        path: "/_health",
        healthyHttpCodes: "200",
        timeout: Duration.seconds(5),
        interval: Duration.seconds(30)
      }
    });

    new CfnOutput(this, "ServiceUrl", {
      value: `http://${loadBalancer.loadBalancerDnsName}`
    });
    new CfnOutput(this, "LoadBalancerDnsName", {
      value: loadBalancer.loadBalancerDnsName
    });
    new CfnOutput(this, "AuroraEndpoint", {
      value: dbCluster.clusterEndpoint.hostname
    });
    new CfnOutput(this, "AuroraPort", {
      value: dbCluster.clusterEndpoint.port.toString()
    });
    new CfnOutput(this, "AuroraSecretArn", {
      value: dbCluster.secret?.secretArn ?? ""
    });
  }
}
