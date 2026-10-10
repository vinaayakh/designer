---
id: practice.aws-s3-gateway-endpoint
type: practice
domain: security
title: Private S3 access through a VPC gateway endpoint
summary: Workloads in an AWS VPC reach S3 through a gateway endpoint, not via NAT or the internet; free, same-Region only.
aliases: [VPC endpoint, gateway endpoint, PrivateLink, interface endpoint, private connectivity, S3, NAT gateway, egress, bucket policy, aws:SourceVpce, endpoint policy, private subnet]
coverage: checkable
applies_to:
  kinds: ["data.object"]
  clouds: [aws]
sources:
  - title: "Gateway endpoints for Amazon S3, Amazon VPC AWS PrivateLink guide"
    url: https://docs.aws.amazon.com/vpc/latest/privatelink/vpc-endpoints-s3.html
    as_of: 2026-10-10
    licence_tier: facts-only
    copy: kb/sources/aws/vpc-endpoints-s3.txt
revised: 2026-10-10
---

## Practice

When compute inside a VPC reads or writes S3, route that traffic through an S3 gateway endpoint attached to the private subnets' route tables. Traffic stays on the AWS network, private subnets need no NAT or internet gateway just for S3, and gateway endpoints cost nothing (NAT data-processing charges disappear for that traffic).

Pair it with a bucket policy that only accepts requests arriving through the endpoint (`aws:SourceVpce`), and an endpoint policy that limits which buckets the VPC can reach.

Limits to know: a gateway endpoint works only for S3 in the same Region. It cannot be used from on-premises, a peered VPC in another Region, or through a Transit Gateway, VPN or Direct Connect; those need an S3 interface endpoint, which is billed. IP-based bucket conditions stop matching, because S3 sees the private source address.

## How to check on a diagram

- Compute in a private subnet Group has an edge to an S3 Node: is there an S3 gateway endpoint on the VPC, or does the edge pass through a NAT gateway?
- Access from on-premises or another Region to S3: an interface endpoint is needed instead.

## Suggested fixes

- Add an S3 gateway endpoint to the VPC Group and route the S3 edges through it.
- Note the bucket policy restriction in the Design Rationale.

## Trade-offs

- Free and simple, but same-Region and VPC-only.
- Creating the endpoint swaps routes and drops open S3 connections, so roll it out in a quiet window.
