package main

import (
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

func HandleClientRequest(c *gin.Context) {
	svr, err := BalanceWeightRandom()
	if err != nil {
		c.JSON(500, gin.H{"msg": "can not get a server"})
		return
	}
	remote, err := url.Parse(svr.Infos.ReqURL)
	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.ServeHTTP(c.Writer, c.Request)
}
