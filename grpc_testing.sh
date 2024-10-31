#!/bin/bash

for i in {1..1000}
do
  go test -race -count=1 ./pkg/remote/trans/nphttp2/grpc/...
done