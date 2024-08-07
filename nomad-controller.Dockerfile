FROM golang:latest AS go-builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ./nomad-controller ./cmd/nomad-controller/main.go

FROM node:20.11 AS frontend-builder
ENV PATH=/build/node_modules/.bin:$PATH
WORKDIR /build
COPY nomad-frontend .
RUN npm i
RUN npm run build

FROM alpine:latest
WORKDIR /app
COPY --from=go-builder /build/nomad-controller .
COPY --from=frontend-builder /build/public ./public
ENTRYPOINT ["./nomad-controller"]

# TODO: custom dockerignores not working?
