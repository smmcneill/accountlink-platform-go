import {
  CfnOutput,
  Duration,
  Stack,
  StackProps
} from "aws-cdk-lib";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as elbv2 from "aws-cdk-lib/aws-elasticloadbalancingv2";
import * as logs from "aws-cdk-lib/aws-logs";
import * as rds from "aws-cdk-lib/aws-rds";
import { Construct } from "constructs";

export interface ServiceStackProps extends StackProps {
  appName: string;
  envName: string;
  imageAssetPath: string;
  imageAssetDockerfile: string;
  containerPort: number;
  desiredCount: number;
  vpc: ec2.IVpc;
  dbCluster: rds.DatabaseCluster;
  dbSecurityGroup: ec2.ISecurityGroup;
  databaseName: string;
}

export class ServiceStack extends Stack {
  constructor(scope: Construct, id: string, props: ServiceStackProps) {
    super(scope, id, props);

    const cluster = new ecs.Cluster(this, "Cluster", {
      vpc: props.vpc,
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
      vpc: props.vpc,
      allowAllOutbound: true,
      description: "ALB security group"
    });
    albSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80), "Allow HTTP");

    const serviceSecurityGroup = new ec2.SecurityGroup(this, "ServiceSg", {
      vpc: props.vpc,
      allowAllOutbound: true,
      description: "ECS service security group"
    });
    serviceSecurityGroup.addIngressRule(albSecurityGroup, ec2.Port.tcp(props.containerPort), "Allow ALB to app");

    new ec2.CfnSecurityGroupIngress(this, "DbIngressFromService", {
      groupId: props.dbSecurityGroup.securityGroupId,
      ipProtocol: "tcp",
      fromPort: 5432,
      toPort: 5432,
      sourceSecurityGroupId: serviceSecurityGroup.securityGroupId,
      description: "Allow ECS service to Postgres"
    });

    const service = new ecs.FargateService(this, "Service", {
      cluster,
      taskDefinition,
      desiredCount: props.desiredCount,
      minHealthyPercent: 100,
      maxHealthyPercent: 200,
      assignPublicIp: false,
      securityGroups: [serviceSecurityGroup],
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS
      }
    });

    container.addEnvironment("DB_HOST", props.dbCluster.clusterEndpoint.hostname);
    container.addEnvironment("DB_PORT", props.dbCluster.clusterEndpoint.port.toString());
    container.addEnvironment("DB_NAME", props.databaseName);
    container.addEnvironment("DB_SSL_MODE", "require");

    if (props.dbCluster.secret) {
      props.dbCluster.secret.grantRead(taskDefinition.taskRole);
      if (taskDefinition.executionRole) {
        props.dbCluster.secret.grantRead(taskDefinition.executionRole);
      }
      container.addSecret("DB_USER", ecs.Secret.fromSecretsManager(props.dbCluster.secret, "username"));
      container.addSecret("DB_PASSWORD", ecs.Secret.fromSecretsManager(props.dbCluster.secret, "password"));
    }

    const loadBalancer = new elbv2.ApplicationLoadBalancer(this, "Alb", {
      vpc: props.vpc,
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
  }
}
