package main

import (
	"flag"

	"github.com/ylt94/mycache/core"
)

func main() {
	srvType := flag.String("type", "master", "请输入类型")
	masterAddr := flag.String("mastAddr", "http://127.0.0.1:8089", "请输入master地址")
	srvAddr := flag.String("srvAddr", "http://127.0.0.1:8088", "请输入master地址")
	nodeAddr := flag.String("nodeAddr", "http://127.0.0.1:8100", "请输入node地址")
	flag.Parse()
	if *srvType == "master" {
		master := core.NewMaster(*masterAddr)
		service := core.NewService(*srvAddr, master)
		core.ServiceStart(service)
	} else {
		nodeCache := core.NewMCache(*nodeAddr, 2<<10, nil)
		nodeService := core.NewNodeServer(*nodeAddr, nodeCache)
		core.ServerStart(nodeService, *masterAddr)
	}

}
