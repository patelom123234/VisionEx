# Multi-stage build for VisionEx application
FROM golang:1.21-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git protobuf-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf files
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    grpc/grpc.proto

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o visionex grpc/cmd/main.go

# Frontend build stage
FROM node:18-alpine AS frontend-builder

WORKDIR /app/ui

# Copy package files
COPY ui/package*.json ./

# Install dependencies
RUN npm ci --only=production

# Copy source code
COPY ui/ ./

# Build frontend
RUN npm run build

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from backend-builder
COPY --from=backend-builder /app/visionex .

# Copy frontend build from frontend-builder
COPY --from=frontend-builder /app/ui/dist ./ui/dist

# Copy configuration files
COPY grpc/cmd/config*.env ./
COPY env.example ./

# Expose ports
EXPOSE 8080 8081

# Run the application
CMD ["./visionex"] 