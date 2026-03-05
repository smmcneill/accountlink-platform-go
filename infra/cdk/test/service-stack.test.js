const path = require("node:path");
const test = require("node:test");
const cdk = require("aws-cdk-lib");
const { Match, Template } = require("aws-cdk-lib/assertions");
const { FoundationStack } = require("../dist/lib/foundation-stack");
const { ServiceStack } = require("../dist/lib/service-stack");

test("service stack configures SNS env vars and publish permissions", () => {
  const app = new cdk.App();

  const foundation = new FoundationStack(app, "accountlink-dev-foundation", {
    env: {
      account: "111111111111",
      region: "us-east-1"
    },
    appName: "accountlink",
    envName: "dev"
  });

  const service = new ServiceStack(app, "accountlink-dev-service", {
    env: {
      account: "111111111111",
      region: "us-east-1"
    },
    appName: "accountlink",
    envName: "dev",
    imageAssetPath: path.resolve(__dirname, "../../.."),
    imageAssetDockerfile: "Dockerfile",
    containerPort: 8080,
    desiredCount: 1,
    vpc: foundation.vpc,
    dbCluster: foundation.dbCluster,
    dbSecurityGroup: foundation.dbSecurityGroup,
    databaseName: foundation.databaseName,
    eventTopic: foundation.eventTopic
  });

  const template = Template.fromStack(service);

  template.hasResourceProperties("AWS::ECS::TaskDefinition", {
    ContainerDefinitions: Match.arrayWith([
      Match.objectLike({
        Environment: Match.arrayWith([
          Match.objectLike({
            Name: "EVENT_TARGET",
            Value: "sns"
          }),
          Match.objectLike({
            Name: "ACCOUNTLINK_SNS_TOPIC_ARN"
          }),
          Match.objectLike({
            Name: "ACCOUNTLINK_SNS_REGION",
            Value: "us-east-1"
          })
        ])
      })
    ])
  });

  template.hasResourceProperties("AWS::IAM::Policy", {
    PolicyDocument: {
      Statement: Match.arrayWith([
        Match.objectLike({
          Action: "sns:Publish",
          Effect: "Allow"
        })
      ])
    }
  });
});
