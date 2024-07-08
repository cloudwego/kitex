#! /bin/bash

kitex -module github.com/cloudwego/kitex  -gen-path .. ./test.thrift

rm -rf ./mock # not in use, rm it
