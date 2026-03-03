export type EnvironmentName = "dev" | "test" | "prod";

export interface EnvironmentConfig {
  account: string;
  region: string;
  appName: string;
  desiredCount: number;
  containerPort: number;
  vpcId?: string;
}
