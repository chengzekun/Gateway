package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	runServer()
}

func runServer() {
	router := gin.Default()
	router.POST("/regUpdate", HandleServerRegisterAndUpdate)
	router.POST("/clientReq", HandleClientRequest)
	router.GET("/clientReq", HandleClientRequest)
	router.Run(":80")
}
