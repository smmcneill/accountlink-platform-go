#!/usr/bin/env node
import { App } from "aws-cdk-lib";
import { PlatformStack } from "../lib/platform-stack";

function readConfig(app: App): {
  appName: string;
  envName: string;
  imageUri: string;
  containerPort: number;
  desiredCount: number;
  hostedZoneDomain: string;
  recordName: string;
  certificateArn: string;
  vpcId?: string;
} {
  const appName = (app.node.tryGetContext("appName") ?? process.env.APP_NAME ?? "accountlink").toString();
  const envName = (app.node.tryGetContext("envName") ?? process.env.ENV_NAME ?? "dev").toString();
  const imageUri = (app.node.tryGetContext("imageUri") ?? process.env.IMAGE_URI ?? "").toString();
  if (imageUri === "") {
    throw new Error("IMAGE_URI (or -c imageUri=...) is required");
  }

  const hostedZoneDomain = (app.node.tryGetContext("hostedZoneDomain") ?? process.env.HOSTED_ZONE_DOMAIN ?? "").toString();
  if (hostedZoneDomain === "") {
    throw new Error("HOSTED_ZONE_DOMAIN (or -c hostedZoneDomain=...) is required");
  }

  const certificateArn = (app.node.tryGetContext("certificateArn") ?? process.env.CERTIFICATE_ARN ?? "").toString();
  if (certificateArn === "") {
    throw new Error("CERTIFICATE_ARN (or -c certificateArn=...) is required");
  }

  const recordName = (app.node.tryGetContext("recordName") ?? process.env.RECORD_NAME ?? appName).toString();
  const containerPort = Number(app.node.tryGetContext("containerPort") ?? process.env.CONTAINER_PORT ?? "8080");
  const desiredCount = Number(app.node.tryGetContext("desiredCount") ?? process.env.DESIRED_COUNT ?? "1");
  const vpcIdRaw = (app.node.tryGetContext("vpcId") ?? process.env.VPC_ID ?? "").toString();
  const vpcId = vpcIdRaw === "" ? undefined : vpcIdRaw;

  return { appName, envName, imageUri, containerPort, desiredCount, hostedZoneDomain, recordName, certificateArn, vpcId };
}

const app = new App();
const cfg = readConfig(app);

new PlatformStack(app, `${cfg.appName}-${cfg.envName}`, {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION
  },
  ...cfg
});
