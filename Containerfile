FROM golang:1.25.7 AS builder

WORKDIR /project

COPY ..

RUN go mod download

RUN go build ./cmd/main.go

ENTRYPOINT ["./main"]
