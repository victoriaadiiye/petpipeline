FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE
RUN go build -o /app ./cmd/${SERVICE}

FROM alpine:3.20
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
