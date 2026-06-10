FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/signalbox ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/signalbox /app/signalbox
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/signalbox"]
