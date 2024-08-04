FROM golang:latest AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ./nomad-agent ./cmd/nomad-agent/main.go

FROM chromedp/headless-shell:latest
WORKDIR /app
COPY --from=builder /build/nomad-agent .
ENTRYPOINT ["./nomad-agent"]
