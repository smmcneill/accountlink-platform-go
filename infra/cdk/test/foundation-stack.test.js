const test = require("node:test");
const assert = require("node:assert/strict");
const cdk = require("aws-cdk-lib");
const { Template } = require("aws-cdk-lib/assertions");
const { FoundationStack } = require("../dist/lib/foundation-stack");

test("foundation stack provisions Aurora, SNS topic, and outputs", () => {
  const app = new cdk.App();
  const stack = new FoundationStack(app, "accountlink-dev-foundation", {
    env: {
      account: "111111111111",
      region: "us-east-1"
    },
    appName: "accountlink",
    envName: "dev"
  });

  const template = Template.fromStack(stack);

  template.resourceCountIs("AWS::RDS::DBCluster", 1);
  template.resourceCountIs("AWS::SNS::Topic", 1);

  template.hasResourceProperties("AWS::SNS::Topic", {
    TopicName: "accountlink-dev-events"
  });

  template.hasOutput("AuroraEndpoint", {});
  template.hasOutput("AuroraPort", {});
  template.hasOutput("AuroraSecretArn", {});
  template.hasOutput("EventTopicArn", {});

  assert.equal(stack.databaseName, "accountlink");
});
