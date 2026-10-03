# Deployment Guide

Flood Watch Ongkharak uses a containerized stack for production deployment. The `docker-compose.prod.yml` configuration securely isolates the database, Redis, MinIO, and AI services from the public internet, exposing only the Caddy reverse proxy which terminates TLS via Let's Encrypt.

## Prerequisites

1. A virtual machine or server with at least 2GB RAM and a public IP address.
2. Docker and Docker Compose installed.
3. A domain name (e.g., `floodwatch.example.com`) pointed to the server's public IP.

## Pre-deployment Preparation

Google login is optional. To enable it, configure the four backend settings and exact HTTPS callback described in `docs/architecture/google-login.md`, and apply migration `000013_google_auth` before deploying the API. Leave all four settings empty to retain password-only authentication. Never expose the Google client secret in frontend settings. The production proxy preserves `/api` for the Go callback.

Before running the stack, you must define the required production secrets. Do not use development-only secrets.

Create a `.env` file in the project root:

```env
DOMAIN_NAME=floodwatch.example.com
DB_PASSWORD=your_secure_postgres_password
MINIO_ACCESS_KEY=admin
MINIO_SECRET_KEY=your_secure_minio_password
CORS_ALLOWED_ORIGINS=https://floodwatch.example.com
JWT_SECRET=your_32_byte_secure_jwt_secret
VAPID_PUBLIC_KEY=your_vapid_public_key
VAPID_PRIVATE_KEY=your_vapid_private_key
VAPID_SUBSCRIBER=mailto:admin@example.com
```

### Generating Secrets

Only `JWT_SECRET` is required for token configuration. The production backend defaults to issuer `floodwatch-ongkharak`, 15-minute access tokens, and 7-day refresh tokens. If an existing deployment uses a different issuer, retain its optional `JWT_ISSUER` setting to keep active access tokens valid.

1. **JWT Secret**: Use a secure random string (at least 32 bytes).
   `openssl rand -hex 32`
2. **VAPID Keys**: Used for push notifications. Generate using standard tools or `npx web-push generate-vapid-keys`.
3. **Database & MinIO passwords**: Use strong generated passwords.

## Build and Start

1. Compile the React frontend so it can be served statically by Caddy.
   ```bash
   cd apps/web
   pnpm install
   pnpm run build
   ```

2. Start the production cluster:
   ```bash
   cd ../../
   docker compose -f docker-compose.prod.yml up -d --build
   ```

## Production Security Notes

- **Network Isolation**: Only port 80 and 443 are exposed. PostgreSQL, Redis, and MinIO are confined to the Docker bridge network.
- **TLS**: Caddy automatically provisions and renews TLS certificates via Let's Encrypt for the domain specified in `DOMAIN_NAME`.
- **Swagger UI**: Disabled automatically when `APP_ENV=production`.
- **Metrics**: Prometheus metrics are available internally on the `api:8080/metrics` endpoint. If you want to scrape them from a remote Prometheus instance, you should configure a secure tailscale/wireguard tunnel or reverse proxy it with basic auth.

## Running Database Migrations

You will need to run the migrations against the production database container. Since the PostgreSQL port is not exposed to the host, you can run the Go backend migration script or a standalone migration container inside the Docker network.
