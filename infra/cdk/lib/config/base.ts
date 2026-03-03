export interface BaseConfig {
  appName: string;
  containerPort: number;
}

export const baseConfig: BaseConfig = {
  appName: "accountlink",
  containerPort: 8080
};
