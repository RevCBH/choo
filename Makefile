.PHONY: proto
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/choo/v1/daemon.proto
	mv proto/choo/v1/*.go pkg/api/v1/
