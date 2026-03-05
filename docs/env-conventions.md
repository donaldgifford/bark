# Environment Variable Conventions

All environment variables for bark follow a consistent naming convention.
This document is the authoritative reference. See `.env.local.example` for
concrete values.

## Naming Rules

| Rule | Example |
|------|---------|
| All caps with underscores | `BARK_API_ADDR` |
| Component prefix for bark-specific vars | `BARK_` |
| Third-party tool vars use their own convention | `AWS_`, `WIZCLI_`, `GRYPE_` |
| Boolean vars use `true`/`false` | `BARK_DEBUG=true` |
| Duration vars use a `_MINUTES` or `_SECONDS` suffix | `BARK_PRESIGNED_URL_TTL_MINUTES` |

## Variable Reference

### Infrastructure

| Variable | Default | Description |
|----------|---------|-------------|
| `BARK_S3_BUCKET` | `homebrew-bottles` | S3 bucket for all package artifacts |
| `BARK_S3_ENDPOINT` | _(empty)_ | Override S3 endpoint for LocalStack (`http://localhost:4566`) |
| `AWS_DEFAULT_REGION` | `us-east-1` | AWS region |
| `AWS_ACCESS_KEY_ID` | _(required)_ | AWS access key (`test` for LocalStack) |
| `AWS_SECRET_ACCESS_KEY` | _(required)_ | AWS secret key (`test` for LocalStack) |

### API Server

| Variable | Default | Description |
|----------|---------|-------------|
| `BARK_API_ADDR` | `:8080` | Address and port the API listens on |
| `BARK_DB_DSN` | _(required)_ | PostgreSQL connection string |
| `BARK_PRESIGNED_URL_TTL_MINUTES` | `5` | Lifetime of presigned S3 download URLs |

### Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `BARK_OIDC_ISSUER` | _(required)_ | OIDC provider issuer URL (used to discover JWKS endpoint) |
| `BARK_OIDC_AUDIENCE` | `bark-api` | Expected `aud` claim in user JWTs |
| `BARK_OIDC_CLIENT_ID` | `bark-cli` | OIDC client ID for the CLI device flow |

### Pipeline

| Variable | Default | Description |
|----------|---------|-------------|
| `BARK_PIPELINE_TOKEN` | _(required)_ | Service token presented by CI to the registration endpoint |
| `BARK_SCANNER` | `grype` | Vulnerability scanner: `grype` or `wiz` |
| `GRYPE_FAIL_ON_SEVERITY` | `critical` | Minimum severity that fails the grype scan |
| `WIZCLI_TOKEN` | _(required if BARK_SCANNER=wiz)_ | Wiz service account token |
| `WIZCLI_API_URL` | `https://api.wiz.io` | Wiz API endpoint |
| `WIZCLI_POLICY_NAME` | _(required if BARK_SCANNER=wiz)_ | Wiz vulnerability policy name |
| `BARK_INTERNAL_ORIGIN_ALLOWLIST` | _(required)_ | Comma-separated URL prefixes for allowed internal bottle origins |
| `BARK_COSIGN_KEY_PATH` | _(required in pipeline)_ | Path to the cosign private key file |

### CLI (pkgtool)

| Variable | Default | Description |
|----------|---------|-------------|
| `BARK_API_URL` | `http://localhost:8080` | API base URL used by pkgtool |

### Notifications

| Variable | Default | Description |
|----------|---------|-------------|
| `BARK_SLACK_WEBHOOK_URL` | _(empty)_ | Slack webhook for approval notifications; leave empty to disable |

## Local Development

Copy `.env.local.example` to `.env.local` and source it before running:

```bash
cp .env.local.example .env.local
# edit .env.local with your values
source .env.local
make infra-up
```

The docker-compose setup uses variables from the shell environment, so
sourcing `.env.local` before running `docker compose` propagates them
correctly.
