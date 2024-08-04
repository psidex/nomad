FROM golang:latest AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ./nomad-controller ./cmd/nomad-controller/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /build/nomad-controller .
ENTRYPOINT ["./nomad-controller"]
