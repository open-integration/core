protoc  api/proto/v1/core.proto --go_out=plugins=grpc:pkg/api/v1/ --proto_path=api/proto/v1
protoc api/proto/v1/core.proto --go_out . --go-grpc_out .