import {
  CfnOutput,
  Duration,
  Stack,
  StackProps
} from "aws-cdk-lib";
import {
  Certificate
} from "aws-cdk-lib/aws-certificatemanager";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as elbv2 from "aws-cdk-lib/aws-elasticloadbalancingv2";
import * as logs from "aws-cdk-lib/aws-logs";
import * as route53 from "aws-cdk-lib/aws-route53";
import * as targets from "aws-cdk-lib/aws-route53-targets";
import { Construct } from "constructs";

export interface PlatformStackProps extends StackProps {
  appName: string;
  envName: string;
  imageUri: string;
  containerPort: number;
  desiredCount: number;
  hostedZoneDomain: string;
  recordName: string;
  certificateArn: string;
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

    taskDefinition.addContainer("App", {
      image: ecs.ContainerImage.fromRegistry(props.imageUri),
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
    albSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(443), "Allow HTTPS");

    const serviceSecurityGroup = new ec2.SecurityGroup(this, "ServiceSg", {
      vpc,
      allowAllOutbound: true,
      description: "ECS service security group"
    });
    serviceSecurityGroup.addIngressRule(albSecurityGroup, ec2.Port.tcp(props.containerPort), "Allow ALB to app");

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

    const loadBalancer = new elbv2.ApplicationLoadBalancer(this, "Alb", {
      vpc,
      internetFacing: true,
      securityGroup: albSecurityGroup
    });

    const certificate = Certificate.fromCertificateArn(this, "Certificate", props.certificateArn);
    const listener = loadBalancer.addListener("HttpsListener", {
      port: 443,
      certificates: [certificate],
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

    const zone = route53.HostedZone.fromLookup(this, "HostedZone", {
      domainName: props.hostedZoneDomain
    });

    new route53.ARecord(this, "AppAlias", {
      zone,
      recordName: props.recordName,
      target: route53.RecordTarget.fromAlias(new targets.LoadBalancerTarget(loadBalancer))
    });

    const fqdn = props.recordName === "" ? props.hostedZoneDomain : `${props.recordName}.${props.hostedZoneDomain}`;
    new CfnOutput(this, "ServiceUrl", {
      value: `https://${fqdn}`
    });
    new CfnOutput(this, "LoadBalancerDnsName", {
      value: loadBalancer.loadBalancerDnsName
    });
  }
}
