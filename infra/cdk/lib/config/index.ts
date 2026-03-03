import { devConfig } from "./dev";
import { prodConfig } from "./prod";
import { testConfig } from "./test";
import { EnvironmentConfig, EnvironmentName } from "./types";

const environments: Record<EnvironmentName, EnvironmentConfig> = {
  dev: devConfig,
  test: testConfig,
  prod: prodConfig
};

export function resolveEnvironmentName(raw: string): EnvironmentName {
  const normalized = raw.trim().toLowerCase();
  if (normalized === "dev" || normalized === "test" || normalized === "prod") {
    return normalized;
  }
  throw new Error(`Invalid envName "${raw}". Expected one of: dev, test, prod`);
}

export function getEnvironmentConfig(envName: EnvironmentName): EnvironmentConfig {
  return environments[envName];
}
