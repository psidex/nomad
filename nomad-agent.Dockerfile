FROM golang:latest AS go-builder
WORKDIR /build
COPY .git .git
COPY cmd cmd
COPY internal internal
COPY go.mod .
COPY go.sum .
COPY LICENSE .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=true -o ./nomad-agent ./cmd/nomad-agent

FROM chromedp/headless-shell:latest
RUN apt update && apt install ca-certificates -y && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=go-builder /build/nomad-agent .
ENTRYPOINT ["./nomad-agent"]
