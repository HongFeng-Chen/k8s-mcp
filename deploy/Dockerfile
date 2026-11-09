# k8s-mcp-server Dockerfile（多阶段构建，非 root、最小化镜像）
FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/k8s-mcp-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/k8s-mcp-server /app/k8s-mcp-server
# 配置由 Helm ConfigMap 挂载到 /app/configs
EXPOSE 8080
ENTRYPOINT ["/app/k8s-mcp-server"]
