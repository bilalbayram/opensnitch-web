# Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build backend
FROM golang:1.22-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags "-X github.com/evilsocket/opensnitch-web/internal/version.Version=${VERSION} -X github.com/evilsocket/opensnitch-web/internal/version.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" -o /opensnitch-web ./cmd/opensnitch-web

# Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /opensnitch-web .
COPY config.yaml.example .
EXPOSE 8080 50051
CMD ["./opensnitch-web", "-config", "config.yaml"]
