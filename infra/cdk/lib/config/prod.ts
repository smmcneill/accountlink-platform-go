import { baseConfig } from "./base";
import { EnvironmentConfig } from "./types";

export const prodConfig: EnvironmentConfig = {
  ...baseConfig,
  account: "333333333333",
  region: "us-east-1",
  desiredCount: 2
};
