import { baseConfig } from "./base";
import { EnvironmentConfig } from "./types";

export const testConfig: EnvironmentConfig = {
  ...baseConfig,
  account: "222222222222",
  region: "us-east-1",
  desiredCount: 1
};
