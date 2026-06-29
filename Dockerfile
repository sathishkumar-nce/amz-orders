# Build stage
FROM golang:1.25.8-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/bin/server cmd/server/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies:
# - ca-certificates for HTTPS requests
# - tzdata for IST scheduling
# - postgresql16-client for pg_dump backups
RUN apk --no-cache add ca-certificates tzdata postgresql16-client

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /home/appuser

# Copy binary from builder
COPY --from=builder /app/bin/server ./server

# Copy runtime data files required by the application
COPY --from=builder /app/SKU_V8.csv ./SKU_V8.csv
COPY --from=builder /app/spreadsheet.json ./spreadsheet.json
COPY --from=builder /app/sample_orders.json ./sample_orders.json

# Change ownership
RUN chown -R appuser:appuser /home/appuser

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./server"]
