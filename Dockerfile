# Stage 1: Build Go binary + WASM
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod ./
COPY go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy
# Build main binary
RUN CGO_ENABLED=1 go build -o portal ./cmd/portal/
# Build WASM module
RUN GOOS=js GOARCH=wasm go build -o static/app.wasm ./wasm/metrics/
# Copy WASM runtime
RUN cp $(go env GOROOT)/misc/wasm/wasm_exec.js static/

# Stage 2: Backend image
FROM alpine:3.19 AS backend
RUN apk add --no-cache docker-cli
WORKDIR /app
COPY --from=builder /app/portal .
COPY --from=builder /app/static ./static
RUN mkdir -p data
CMD ["./portal"]

# Stage 3: Nginx image
FROM nginx:alpine AS nginx
COPY nginx/nginx.conf /etc/nginx/conf.d/default.conf
