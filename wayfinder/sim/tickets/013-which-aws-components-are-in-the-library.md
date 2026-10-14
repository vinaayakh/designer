---
id: "013"
title: "Which AWS components are in the library, and how are they grouped?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["012"]
parent: map
---

# Which AWS components are in the library, and how are they grouped?

## Question

vinu: the library should be **searchable** and **group components** by things like **load balancer, compute, ECS, EKS**. Which AWS Components go in, in which groups, and what does each one say about simulation?

## Resolution

Decided 2026-10-15. The library is [internal/catalog/library/aws.yaml](../../../internal/catalog/library/aws.yaml): 83 Components in 13 groups plus Quick inserts, with every other catalog Service findable under **More services**.

**Groups** follow how people think about AWS, not AWS's own 24 icon categories. ECS and EKS get their own groups, as vinu asked. In display order:

| Group | Components |
|---|---|
| Quick inserts | Cache in front, Queue between, Read replica, Load balancer in front, CDN in front, Circuit breaker (one-drop changes with wiring rules, from ticket 008) |
| Traffic sources | Users, Mobile app, API client, Internet: where Routes start |
| Load balancing | ALB, NLB, Gateway Load Balancer, Global Accelerator |
| Edge and APIs | CloudFront, API Gateway, AppSync, Route 53 |
| Compute | EC2 Auto Scaling group, EC2 instance, Lambda, App Runner, Elastic Beanstalk, Lightsail, Batch |
| ECS | ECS cluster (Fargate), ECS cluster (EC2 capacity), ECS service on Fargate, ECS service on EC2, ECS task |
| EKS | EKS cluster (managed node group), EKS cluster (Fargate profile), EKS workload (Deployment) |
| Databases | RDS (PostgreSQL, MySQL, MariaDB, SQL Server, Oracle), Aurora (PostgreSQL, MySQL, DSQL), DynamoDB, DocumentDB, Keyspaces, Neptune, OpenSearch, Timestream |
| Caching | ElastiCache (Valkey, Redis OSS, Memcached), MemoryDB, DAX |
| Messaging and streaming | SQS, SNS, EventBridge, Kinesis Data Streams, MSK, Amazon MQ, Step Functions, Data Firehose |
| Storage | S3, S3 Glacier, EFS, FSx, Backup |
| Network | Region, Availability Zone, VPC, public and private subnet, AWS account (Groups); internet and NAT gateways, VPC endpoints, PrivateLink, Transit Gateway, peering, VPN, Direct Connect |
| Security and identity | WAF, Shield, Network Firewall, Cognito, KMS, Secrets Manager |
| Monitoring | CloudWatch, CloudWatch Logs, X-Ray, CloudTrail |

**Simulation coverage is shown on every Component:**
- **Simulated** (`full`): it has capacity from a Behaviour Profile.
- **Pass-through** (`hop`): it sits on the request path and adds latency and its quota, but has no capacity of its own (WAF, NAT gateway, endpoints, Cognito, Gateway Load Balancer).
- **Diagram only** (`off-path`): not in the request flow (Route 53, KMS, CloudWatch, Backup). It's dimmed in Sim Mode, and traffic never routes through it.

"Simulated" and "Pass-through" both need a Behaviour Profile for the Kind. All 31 Kinds that AWS Components (and third-party APIs) use on the request path have one (ticket 011, including its second batch). A lint fails if a Simulated or Pass-through Component's Kind has no Profile; traffic sources and Groups are exempt.

**Catalog additions** (overriding [v1 ticket 017](../../tickets/017-what-are-the-component-kinds-and-cloud-catalog-contents.md)'s Service list):
- `aws.eks-workload` (Kind `compute.container`): pods of a Kubernetes Deployment on EKS. `aws.eks` stays the cluster (Kind `compute.kubernetes`).
- `azure.aks-workload`, the same for AKS.

**Order and defaults.** Each group lists its most-used Component first. Defaults follow AWS best practice:
- web tiers start at 2 instances across 2 zones with autoscaling at 60%;
- databases at one size with Multi-AZ;
- the cache is Valkey, AWS's current default engine.
