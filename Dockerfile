# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o crowdsec-capi .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/crowdsec-capi /crowdsec-capi
ENTRYPOINT ["/crowdsec-capi"]
CMD ["serve"]
