package proxy

import (
	"Gateway/svrpool"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	retryTimes = 2
)

func Proxy(c *gin.Context) {
	serviceName := c.Query("service")
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "can not read request body", "rsp": nil})
		return
	}
	schedulerInstance, ok := svrpool.SchedulerPool.Load(serviceName)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "can not get scheduler instance", "rsp": nil})
		return
	}
	var scheduler svrpool.Scheduler
	scheduler, ok = schedulerInstance.(svrpool.Scheduler)

	var invoker svrpool.Invoker
	for i := 0; i < retryTimes; i++ {
		invoker, err = scheduler.Select()
		if err != nil {
			continue

		}
		var rsp []byte
		rsp, err = invoker.Invoke(body)
		if err != nil {
			continue

		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "Success", "rsp": string(rsp)})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": fmt.Sprintf("%v", err), "rsp": nil})
	return
}
