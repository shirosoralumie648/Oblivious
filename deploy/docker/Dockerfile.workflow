FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY src/server/go.mod src/server/go.sum ./
RUN go mod download
COPY src/server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /workflow ./cmd/workflow

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /workflow /workflow
EXPOSE 8082
CMD ["/workflow"]
