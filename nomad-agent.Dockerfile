FROM golang:latest AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=true -o ./nomad-agent ./cmd/nomad-agent/main.go

FROM chromedp/headless-shell:latest
RUN apt update && apt install ca-certificates -y && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /build/nomad-agent .
ENTRYPOINT ["./nomad-agent"]
