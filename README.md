# fserver-udp
udp file server, transfer files over udp socket.

this project uses protobuf to encode and decode messages: https://protobuf.dev/
see golang implementation in https://pkg.go.dev/google.golang.org/protobuf

## Protocol

The protocol are divided in two sections, first byte is reserved to the packet identification byte, next bytes are reserved for data.
Check [protobuf file](./messages.proto) to known which type data are encoded.
```protobuf
  Server packet: [Header|Protobuf...] 1024 bytes
  header byte types:
  1 -> RESPONSE     // file chunk
  2 -> CONFIRMATION // result
  ...1024 bytes -> Protobuf encoded data

  Client packet: [Header|Protobuf...] 32 bytes
  header byte types:
  0 -> REQUEST      // file
  2 -> CONFIRMATION // result
  ...31 bytes -> Protobuf encoded data
```

## Client
in order to create a client implementation you must send a request message to server containing the file name, then 
start receiving file chunks from server and send confirmation for each packet, server will wait to send next packets
until a confirmation packet, if a packet was losted, client must send a missing packet confirmation and then server 
will return sending next packets. After all packets was received, now you can reconstruct file and generate sha256 
checksum to validate it, if validation succeed you must send a confirmation of the checksum to the server. All needed
messages types are described in [protobuf file](./messages.proto), so every programming languague that has protocol 
buffers compiler could be a client.

## Communication flow
![flux](https://github.com/Fabiokleis/fserver-udp/assets/66813406/b078467e-ee14-4a44-8fc0-e47ff57fff95)


## Docker
to build and test easilly just run:
```shell
docker build -t fserver-udp .
```
```shell
docker container run -d -p 2224:2224 --name fserver fserver-udp
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
