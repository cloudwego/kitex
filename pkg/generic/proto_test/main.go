package main

import (
	kitex_gen "github.com/cloudwego/kitex/pkg/generic/proto_test/kitex_gen/kitex_gen/exampleservice"
	"log"
)

func main() {
	svr := kitex_gen.NewServer(new(ExampleServiceImpl))

	err := svr.Run()

	if err != nil {
		log.Println(err.Error())
	}
}
