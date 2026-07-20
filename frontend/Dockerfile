# syntax=docker/dockerfile:1
# Build context: frontend/  (docker build -t bubblepulse-frontend:local dailybot/frontend/)

# ─── Stage 1: Build Vue app ───────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

# ─── Stage 2: Serve with Nginx ────────────────────────────────────────────────
FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
