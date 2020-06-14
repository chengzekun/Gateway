package main

import (
	"Gateway/proxy"
	"Gateway/sortsvr"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	router.POST("/sortServer", sortsvr.ContactSortServer)
	router.POST("/sortService", proxy.Proxy)
	router.Run(":80")
}
