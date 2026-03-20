```mermaid
flowchart TB
    %% External
    Internet[(Internet Users)]

    %% Foundation stack
    subgraph Foundation["Foundation Stack"]
        direction TB

        EventTopic["SNS Topic<br/>accountlink-dev-events"]
        DbSecret["Secrets Manager<br/>Aurora credentials"]

        subgraph VPC["VPC 10.0.0.0/16"]
            direction TB

            IGW["Internet Gateway"]

            %% Public
            subgraph Public["Public Subnets"]
                direction LR
                PubA["PublicSubnet1<br/>us-east-1a"]
                PubB["PublicSubnet2<br/>us-east-1b"]

                NAT["NAT Gateway<br/>(in PublicSubnet1)"]
                ALB["Application Load Balancer<br/>internet-facing"]
            end

            %% Private
            subgraph Private["Private Subnets"]
                direction LR
                PrivA["PrivateSubnet1<br/>us-east-1a"]
                PrivB["PrivateSubnet2<br/>us-east-1b"]

                TaskA["Fargate Task<br/>App :8080"]
                TaskB["Fargate Task<br/>App :8080"]
            end

            %% Data
            subgraph Data["Data Layer"]
                direction TB
                DbSubnetGroup["DB Subnet Group<br/>private subnets"]
                Aurora["Aurora PostgreSQL<br/>Serverless v2"]
                Writer["Writer Instance"]
            end

            %% Security groups
            AlbSg["ALB SG<br/>:80 from 0.0.0.0/0"]
            ServiceSg["Service SG<br/>:8080 from ALB SG"]
            DbSg["DB SG<br/>:5432 from Service SG"]
        end
    end

    %% Service stack
    subgraph Service["Service Stack"]
        direction TB

        Cluster["ECS Cluster<br/>accountlink-dev"]
        TaskDef["Task Definition<br/>512 CPU / 1024 MB"]
        TaskRole["Task Role<br/>read secret<br/>publish SNS"]
        ExecRole["Execution Role<br/>ECR + logs + secret"]
        TG["Target Group<br/>HTTP :8080"]
        Listener["Listener<br/>HTTP :80"]
        Logs["CloudWatch Logs"]
        Image["ECR Image"]
    end

    %% Ingress
    Internet -->|"HTTP :80"| ALB
    IGW --> ALB
    ALB --> Listener
    Listener --> TG
    TG --> TaskA
    TG --> TaskB

    %% Security relationships
    AlbSg -.-> ALB
    ServiceSg -.-> TaskA
    ServiceSg -.-> TaskB
    AlbSg -->|"allow :8080"| ServiceSg
    ServiceSg -->|"allow :5432"| DbSg
    DbSg -.-> Aurora

    %% ECS relationships
    Cluster --> TaskDef
    TaskDef --> TaskA
    TaskDef --> TaskB

    %% Runtime dependencies
    ExecRole --> Image
    ExecRole --> Logs
    ExecRole --> DbSecret

    TaskRole --> DbSecret
    TaskRole --> EventTopic

    TaskA -->|"read secret"| DbSecret
    TaskB -->|"read secret"| DbSecret

    TaskA -->|"Postgres :5432"| Aurora
    TaskB -->|"Postgres :5432"| Aurora

    TaskA -->|"publish"| EventTopic
    TaskB -->|"publish"| EventTopic

    %% Data layer
    DbSubnetGroup --> Aurora
    Aurora --> Writer

    %% Egress
    NAT -->|"outbound internet"| TaskA
    NAT -->|"outbound internet"| TaskB
    ```