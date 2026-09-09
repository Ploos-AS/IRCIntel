# Architecture

IRCIntel is designed as a distributed IRC observability and intelligence platform.

## Components

### Core

The central service owns network metadata, scheduling, aggregation, incident detection, API delivery, and historical data.

### Agent

A later milestone introduces a lightweight distributed probe that can run on small VPS nodes. Agents will perform DNS, TCP, TLS and IRC protocol measurements and submit normalized observations to Core.

### Web

The public web UI will consume the same API exposed by Core.

## Initial data model

Future milestones will introduce explicit entities for:

- networks
- servers/endpoints
- probe locations
- observations
- IRCv3 capabilities
- incidents and netsplits
- public channels
- historical aggregates

## Deployment principles

- OCI-first
- amd64 and arm64
- rootless/non-root friendly
- minimal runtime image
- Docker and Podman support
- suitable for low-resource VPS nodes
