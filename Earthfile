VERSION 0.8

FROM golang:1.26.1-alpine

# install gcc dependencies into alpine for CGO
RUN apk --no-cache add git ca-certificates gcc musl-dev libc-dev binutils-gold curl openssh

# install docker tools
# https://docs.docker.com/engine/install/debian/
RUN apk add --update --no-cache docker

# install the go generator plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN export PATH="$PATH:$(go env GOPATH)/bin"

# install buf from source
RUN GO111MODULE=on GOBIN=/usr/local/bin go install github.com/bufbuild/buf/cmd/buf@v1.66.1

# install the various tools to generate connect-go
RUN go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
RUN go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

# install linter
# binary will be $(go env GOPATH)/bin/golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.11.4
RUN golangci-lint --version

# install vektra/mockery
RUN go install github.com/vektra/mockery/v2@v2.53.2


protogen:
    # copy the proto files to generate
    COPY --dir protos/ ./
    COPY buf.yaml buf.gen.yaml ./

    # generate the Go code (.pb.go and _grpc.pb.go)
    RUN buf generate \
            --template buf.gen.yaml \
            --path protos/egrpc

    # generate the binary descriptor set (.binpb) for the gRPC egress tests
    RUN buf build \
            --path protos/egrpc \
            --output gen/egrpc/test_service.binpb

    # copy hand-written test helpers into the gen output after buf generate
    # (buf.gen.yaml has clean:true which wipes gen/ before generating)
    COPY internal/egress/grpc/testdata/server.go gen/egrpc/server.go

    # save artifact to
    SAVE ARTIFACT gen/egrpc AS LOCAL  internal/egress/grpc/testdata
