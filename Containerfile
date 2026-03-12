FROM golang:1.25.7 AS builder

WORKDIR /project

COPY ..

RUN go mod download

CMD ["go", "run", "./cmd"]
