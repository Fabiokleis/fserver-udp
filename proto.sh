#!/bin/sh

echo "Generating GO files..."

protoc \
    --proto_path=./ \
    --go_out=./pkg/proto \
    --go_opt=paths=source_relative \
    ./request.proto \
