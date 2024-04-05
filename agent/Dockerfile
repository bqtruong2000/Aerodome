# build golang server stage
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY . .
RUN go mod tidy
RUN go mod download
RUN go build -o agent ./cmd/agent/main.go


# final stage
FROM alpine:latest
COPY --from=builder /build/agent ./agent
COPY --from=builder /build/agent.yaml ./agent.yaml

# install ca-certificates
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

CMD ["./agent"]