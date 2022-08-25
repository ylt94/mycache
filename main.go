package main

import (
	"github.com/ylt94/mycache/core"
	"log"
	"sync"
)

func main() {
	serviceAddr := "127.0.0.1:8088"
	masterAddr := "127.0.0.1:8089"
	nodeAddr := "127.0.0.1:8100"
	master := core.NewMaster(masterAddr)
	service := core.NewService(serviceAddr, master)

	nodeCache := core.NewMCache(nodeAddr, 2 << 10, nil)
	nodeService := core.NewNodeServer(nodeAddr, nodeCache)
	log.Println(1111111)
	core.ServiceStart(service)
	log.Println(2222222)
	go core.ServerStart(nodeService, masterAddr)
	log.Println(3333333)
	wg := sync.WaitGroup{}
	wg.Wait()
}