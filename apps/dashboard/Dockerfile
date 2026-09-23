# ==============================================================================
# Stage 1: Build React static bundle
# ==============================================================================
FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .
RUN npm run build

# ==============================================================================
# Stage 2: Serve static files with lightweight Nginx
# ==============================================================================
FROM nginx:1.27-alpine

RUN apk upgrade --no-cache

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --chmod=755 40-lensio-config.sh /docker-entrypoint.d/40-lensio-config.sh
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:80/healthz || exit 1

CMD ["nginx", "-g", "daemon off;"]
