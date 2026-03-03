import { baseConfig } from "./base";
import { EnvironmentConfig } from "./types";

export const devConfig: EnvironmentConfig = {
  ...baseConfig,
  account: "207402030994",
  region: "us-east-1",
  desiredCount: 1
};
