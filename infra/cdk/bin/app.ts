#!/usr/bin/env node
import path from "path";
import { App } from "aws-cdk-lib";
import {
  getEnvironmentConfig,
  resolveEnvironmentName
} from "../lib/config";
import { FoundationStack } from "../lib/foundation-stack";
import { ServiceStack } from "../lib/service-stack";

function readConfig(app: App): {
  appName: string;
  envName: "dev" | "test" | "prod";
  imageAssetPath: string;
  imageAssetDockerfile: string;
  containerPort: number;
  desiredCount: number;
  account: string;
  region: string;
  vpcId?: string;
} {
  const envNameRaw = (app.node.tryGetContext("envName") ?? process.env.ENV_NAME ?? "dev").toString();
  const envName = resolveEnvironmentName(envNameRaw);
  const base = getEnvironmentConfig(envName);

  const appName = (app.node.tryGetContext("appName") ?? process.env.APP_NAME ?? base.appName).toString();
  const imageAssetPath = (app.node.tryGetContext("imageAssetPath") ?? process.env.IMAGE_ASSET_PATH ?? path.resolve(__dirname, "../../..")).toString();
  const imageAssetDockerfile = (app.node.tryGetContext("imageAssetDockerfile") ?? process.env.IMAGE_ASSET_DOCKERFILE ?? "Dockerfile").toString();
  const containerPort = Number(app.node.tryGetContext("containerPort") ?? process.env.CONTAINER_PORT ?? base.containerPort.toString());
  const desiredCount = Number(app.node.tryGetContext("desiredCount") ?? process.env.DESIRED_COUNT ?? base.desiredCount.toString());
  const account = (app.node.tryGetContext("account") ?? process.env.AWS_ACCOUNT_ID ?? base.account).toString();
  const region = (app.node.tryGetContext("region") ?? process.env.AWS_REGION ?? base.region).toString();
  const vpcIdRaw = (app.node.tryGetContext("vpcId") ?? process.env.VPC_ID ?? "").toString();
  const vpcId = vpcIdRaw === "" ? undefined : vpcIdRaw;

  return { appName, envName, imageAssetPath, imageAssetDockerfile, containerPort, desiredCount, account, region, vpcId };
}

const app = new App();
const cfg = readConfig(app);
const env = {
  account: cfg.account,
  region: cfg.region
};

const foundation = new FoundationStack(app, `${cfg.appName}-${cfg.envName}-foundation`, {
  env,
  appName: cfg.appName,
  envName: cfg.envName,
  vpcId: cfg.vpcId
});

new ServiceStack(app, `${cfg.appName}-${cfg.envName}-service`, {
  env,
  appName: cfg.appName,
  envName: cfg.envName,
  imageAssetPath: cfg.imageAssetPath,
  imageAssetDockerfile: cfg.imageAssetDockerfile,
  containerPort: cfg.containerPort,
  desiredCount: cfg.desiredCount,
  vpc: foundation.vpc,
  dbCluster: foundation.dbCluster,
  dbSecurityGroup: foundation.dbSecurityGroup,
  databaseName: foundation.databaseName
});
