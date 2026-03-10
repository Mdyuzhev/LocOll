# Stage 1: Build Go binary + WASM
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod ./
COPY go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
ENV GOTOOLCHAIN=auto
RUN go mod tidy
# Build main binary
RUN CGO_ENABLED=1 go build -o portal ./cmd/portal/
# Build WASM module
RUN GOOS=js GOARCH=wasm go build -o static/app.wasm ./wasm/metrics/
# Copy WASM runtime - find it in either base or downloaded toolchain
RUN find / -name wasm_exec.js -path "*/misc/wasm/*" 2>/dev/null | head -1 | xargs -I{} cp {} static/ || \
    wget -q -O static/wasm_exec.js https://raw.githubusercontent.com/nicholasgasior/gowasm/master/wasm_exec.js 2>/dev/null || \
    cp /usr/local/go/misc/wasm/wasm_exec.js static/ 2>/dev/null || true

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
