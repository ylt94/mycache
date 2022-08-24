package main

import "github.com/ylt94/mycache/core"

func main() {
	addr := "127.0.0.1:8088"
	mainCache := core.NewMCache("master",2 << 10, core.GetterFunc(
			func(key string) ([]byte, error) {
				return make([]byte, 0), nil
			}))
	server := core.NewHTTPServer(addr, mainCache)
	core.ServerStart(server)
}