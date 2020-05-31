package main

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
)

type serv struct {
	name string
	lock sync.Mutex
}

func main() {
	var svr serv
	svr.lock.Lock()
	defer svr.lock.Unlock()
	fmt.Println(svr.lock)
}

func runServer() {
	router := gin.Default()
	//router.POST("/regUpdate", HandleServerRegisterAndUpdate)
	//router.POST("/clientReq", HandleClientRequest)
	router.Run(":80")
}
