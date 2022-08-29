package main

import "github.com/ylt94/mycache/core"

func main() {
	//serviceAddr := "http://127.0.0.1:8088"
	//master := core.NewMaster(serviceAddr)
	//service := core.NewService(serviceAddr, master)
	//core.ServiceStart(service)

	masterAddr := "http://127.0.0.1:8088"
	nodeAddr := "http://127.0.0.1:8100"

	nodeCache := core.NewMCache(nodeAddr, 2<<10, nil)
	nodeService := core.NewNodeServer(nodeAddr, nodeCache)
	core.ServerStart(nodeService, masterAddr)
}
