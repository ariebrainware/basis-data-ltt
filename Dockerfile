# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app

# Cache go modules installation
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build the binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app .

# Run stage
FROM alpine:latest
LABEL maintainer="Arie Brainware"
WORKDIR /app

# Copy binary from builder stage.
COPY --from=builder /app/app .

# Expose port if needed (e.g., 8080)
EXPOSE 19091

CMD ["./app"]