.PHONY: all build proto frontend clean run dev embed

all: frontend embed build

# Generate proto files
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/ui.proto

# Build frontend
frontend:
	cd web && npm install && npm run build

# Copy frontend build into Go embed directory
embed: frontend
	rm -rf cmd/opensnitch-web/frontend
	cp -r web/dist cmd/opensnitch-web/frontend

# Build Go binary (with embedded frontend)
build: embed
	CGO_ENABLED=1 go build -o bin/opensnitch-web ./cmd/opensnitch-web

# Run the server (dev mode — serves from web/dist)
run:
	CGO_ENABLED=1 go run ./cmd/opensnitch-web

# Development: run backend + frontend dev server
dev:
	CGO_ENABLED=1 go run ./cmd/opensnitch-web &
	cd web && npm run dev

# Clean build artifacts
clean:
	rm -rf bin/ web/dist web/node_modules cmd/opensnitch-web/frontend

# Docker build
docker:
	docker build -t opensnitch-web .
