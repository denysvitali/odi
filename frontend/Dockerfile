# Build stage
FROM node:24.21.0-alpine AS builder

# Install pnpm (align with CI version)
RUN npm i -g pnpm@11.10.0

WORKDIR /app

# Copy only package files for caching
COPY package*.json ./
COPY pnpm-lock.yaml pnpm-workspace.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile --ignore-scripts

# Copy source code
COPY . .

# Build the app
RUN pnpm run build

# Production stage
FROM nginx:alpine

# Patch base-image OS packages whose fixes are already published in the Alpine
# repos but not yet baked into the upstream nginx:alpine tag. Without this the
# image trails the distro on security fixes (e.g. CVE-2026-6732 in libxml2,
# fixed in 2.13.9-r1) and the Trivy gate in images.yml fails.
RUN apk upgrade --no-cache

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Optional: copy custom entrypoint if needed (keeping existing)
COPY ./docker/entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
