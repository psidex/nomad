FROM golang:latest AS go-builder
WORKDIR /build
COPY .git .git
COPY cmd cmd
COPY internal internal
COPY go.mod .
COPY go.sum .
COPY LICENSE .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=true -o ./nomad-controller ./cmd/nomad-controller

FROM node:20 AS frontend-builder
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
