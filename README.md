# fserver-udp
udp file server, transfer files over udp socket.

this project uses protobuf to enconde and decode messages: https://protobuf.dev/
see golang implementation in https://pkg.go.dev/google.golang.org/protobuf

```env
UDP_BIND_ADDRESS = "0.0.0.0:2224"
```

## Build
setup golang protobuf compiler by running:
```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

generate golang protobuf bindings by running:
```shell
./proto.sh # chmod +x proto.sh
```

## Run
start udp server by running:
```shell
go run ./cmd/fserver-udp/main.go
```
