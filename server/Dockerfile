# Stage 1: Build
FROM golang:alpine AS builder

# Set environment variables for Go
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

WORKDIR /app

# Copy Go modules files and download dependencies
COPY go.mod go.sum ./ 
RUN go mod download

# Copy the entire source code to the container
COPY . .
COPY .env .env
# Build the Go application
RUN go build -o main ./src/

# Stage 2: Development
FROM golang:alpine AS dev

WORKDIR /app

# Install necessary tools for development
RUN apk add --no-cache bash curl

# Copy Go modules files and download dependencies
COPY go.mod go.sum ./ 
RUN go mod download

# Copy the entire source code to the container
COPY . .
COPY .env .env

# Expose development ports
EXPOSE 8080
EXPOSE 5000

# Run the application in development mode with `go run`
CMD ["go", "run", "./src/main.go"]

# Stage 3: Production
FROM alpine:latest AS prod

WORKDIR /app/

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/main .

# Copy the .env file (only if necessary for production runtime)
COPY .env .env

# Expose only production-specific ports
EXPOSE 8080
EXPOSE 5000

# Run the optimized application
CMD ["./main"]
