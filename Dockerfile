FROM golang:1.23 AS builder
WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /dist/backup

# Now copy it into our base image.
FROM alpine:latest AS certs
RUN apk add --no-cache ca-certificates && \
    update-ca-certificates

FROM scratch

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /dist /

ENTRYPOINT ["/backup", "watch"]
