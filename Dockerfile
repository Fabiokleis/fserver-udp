# syntax=docker/dockerfile:1
FROM golang:1.21.8 AS build

WORKDIR /app

COPY . /app

# Protobuf script setup
RUN apt-get update

RUN apt-get install -y protobuf-compiler

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

RUN ./proto.sh

RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /fserver /app/cmd/fserver-udp/main.go

FROM scratch

WORKDIR /app

COPY --from=build /fserver ./

EXPOSE 2224

# Run
CMD ["./fserver"]
