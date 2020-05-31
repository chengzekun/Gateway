package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

type Fields []string

// RegisterOrUpdate 注册或者更新服务器信息时的报文结构
type RegisterOrUpdate struct {
	Op             int32    `json:"op"`             // 操作类型：1：注册，2：更新
	ServerId       uint32   `json:"serverId"`       // 服务器ID，在注册时为空，在更新时需要提供
	SvrInfo        ServInfo `json:"serverInfo"`     // 服务器的信息：CPU个数，内存，请求的链接
	ModifiedFields []string `json:"modifiedFields"` // 发生变更的字段：在更新时提供
}

// Contain 判断变更的字段中是否包含特定字段
func (fields Fields) Contain(field string) bool {
	for _, val := range fields {
		if val == field {
			return true
		}
	}
	return false
}

type Response struct {
	Msg    string   `json:"msg"`
	Fields ServInfo `json:"fields"`
}

const (
	RegisterOp = 1
	UpdateOp   = 2
)

func HandleServerRegisterAndUpdate(c *gin.Context) {
	packet := &RegisterOrUpdate{}
	err := c.BindJSON(packet)
	if err != nil {
		log.Println("[ ERROR ] Parse server infos failed!")
	}

	if packet.Op == RegisterOp {
		// 执行注册过程
		svr, err := Register(packet.SvrInfo)

		if err != nil {
			res := Response{Msg: err.Error()}
			c.JSON(400, res)
			return
		}

		res := Response{Msg: "Succeeded", Fields: svr}
		c.JSON(200, res)
		return
	} else if packet.Op == UpdateOp {
		// 判断是否存在对应的Server
		if svr, exist := EdgeServers.Svrs[packet.ServerId]; exist {
			svr.Update(packet)
			c.JSON(200, Response{Msg: "Succeeded", Fields: svr.Infos})
			return
		}
	}
	c.JSON(400, Response{Msg: "Can not parse right operation"})
	return
}
