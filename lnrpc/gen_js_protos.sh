#!/bin/sh

echo "Generating root gRPC server protos"

PROTOS="rpc.proto walletunlocker.proto **/*.proto"

# For each of the sub-servers, we then generate their protos, but a restricted
# set as they don't yet require REST proxies, or swagger docs.
mkdir lnrpc

for file in $PROTOS; do
  DIRECTORY=$(dirname "${file}")
  echo "Generating protos from ${file}, into ${DIRECTORY}"

  # Generate the protos.
  protoc -I/usr/local/include -I. \
    --js_out=import_style=commonjs:lnrpc \
    --grpc-web_out=import_style=commonjs,mode=grpcwebtext:lnrpc "${file}"

done
