package main

import (
	"context"
	"errors"
	"hash/crc32"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// 描述服务器的信息
type ServInfo struct {
	Id         uint32
	Weight     int32  `json:"weight"`
	InvokeNum  int32  `json:"invokeNum"`
	CoreNum    int32  `json:"coreNum"`
	Memory     int32  `json:"memory"`
	ReqURL     string `json:"reqURL"`
	Shutdown   bool   `json:"shutdown"`
	AvgTime    int64  `json:"avgTime"`
	LastUpdate time.Time
}

var (
	EdgeServers Servers = Servers{RWLock: sync.RWMutex{}, Svrs: make(map[uint32]*Server)}
)

type Servers struct {
	RWLock sync.RWMutex
	Svrs   map[uint32]*Server
}

type Server struct {
	RWLock sync.RWMutex
	Infos  ServInfo
}

func (svr *Server) SetId(id uint32) {
	svr.Infos.Id = id
}
func (svr *Server) SetWeight(wt int32) {
	atomic.StoreInt32(&svr.Infos.Weight, wt)
}
func (svr *Server) AddInvokeNum(delta int32) {
	atomic.AddInt32(&svr.Infos.InvokeNum, delta)
}
func (svr *Server) SetCoreNum(cn int32) {
	atomic.StoreInt32(&svr.Infos.CoreNum, cn)
}
func (svr *Server) SetMemory(mm int32) {
	atomic.StoreInt32(&svr.Infos.Memory, mm)
}
func (svr *Server) SetReqURL(reqUrl string) {
	svr.RWLock.Lock()
	defer svr.RWLock.Unlock()
	svr.Infos.ReqURL = reqUrl
}
func (svr *Server) Touch() {
	svr.Infos.LastUpdate = time.Now()
}
func (svr *Server) SetAvgTime(avg int64) {
	atomic.StoreInt64(&svr.Infos.AvgTime, avg)
}

// Clone 返回Server信息的一个副本
func (svr *Server) Clone() Server {
	svr.RWLock.RLock()
	defer svr.RWLock.RUnlock()
	return Server{RWLock: sync.RWMutex{},
		Infos: ServInfo{Id: svr.Infos.Id,
			Weight:     svr.Infos.Weight,
			CoreNum:    svr.Infos.CoreNum,
			Memory:     svr.Infos.Memory,
			ReqURL:     svr.Infos.ReqURL,
			AvgTime:    svr.Infos.AvgTime,
			Shutdown:   svr.Infos.Shutdown,
			LastUpdate: svr.Infos.LastUpdate}}
}

// 注册时必须要包含：CPU核心数，内存容量，接收请求的URL
func Register(servInfos ServInfo) (ServInfo, error) {
	log.Printf("Server info is %+v", servInfos)
	EdgeServers.RWLock.Lock()
	defer EdgeServers.RWLock.Unlock()

	var svr Server

	// 计算该server 是否已经存在于内存中
	hashId := crc32.ChecksumIEEE([]byte(servInfos.ReqURL))
	if inSvr, ok := EdgeServers.Svrs[hashId]; ok {
		if inSvr.Infos.ReqURL == servInfos.ReqURL {
			return svr.Infos, errors.New("repeat register")
		}
	}
	svr.Infos = servInfos
	svr.Infos.Id = hashId
	svr.Touch()
	EdgeServers.Svrs[hashId] = &svr
	return svr.Infos, nil
}

// 可以更新的参数主要包括：权重，核心数，内存，请求的URL，shutdown，AvgTime
func (svr *Server) Update(updatePacket *RegisterOrUpdate) {

	if Fields(updatePacket.ModifiedFields).Contain("weight") {
		svr.SetWeight(updatePacket.SvrInfo.Weight)
	}

	if Fields(updatePacket.ModifiedFields).Contain("coreNum") {
		svr.SetCoreNum(updatePacket.SvrInfo.CoreNum)
	}

	if Fields(updatePacket.ModifiedFields).Contain("memory") {
		svr.SetMemory(updatePacket.SvrInfo.Memory)
	}

	if Fields(updatePacket.ModifiedFields).Contain("avgTime") {
		svr.SetAvgTime(updatePacket.SvrInfo.AvgTime)
	}

	if Fields(updatePacket.ModifiedFields).Contain("shutdown") {
		EdgeServers.RWLock.Lock()
		defer EdgeServers.RWLock.Unlock()
		delete(EdgeServers.Svrs, updatePacket.ServerId)
		return
	}

	// 如果ReqURL 发生了变更，可能会涉及到修改EdgeServers中的Svrs的key信息
	if Fields(updatePacket.ModifiedFields).Contain("reqURL") {
		EdgeServers.RWLock.Lock()
		defer EdgeServers.RWLock.Unlock()
		delete(EdgeServers.Svrs, updatePacket.ServerId)
		newId := crc32.ChecksumIEEE([]byte(updatePacket.SvrInfo.ReqURL))
		svr.SetId(newId)
		svr.SetReqURL(updatePacket.SvrInfo.ReqURL)
		EdgeServers.Svrs[newId] = svr
	}

	svr.Touch()
}

// CleanNoHeartBeatSvr: 定期清除长时间没有更新的server
func CleanNoHeartBeatSvr(ctx context.Context, sec int, dura int) {
	for {
		select {
		case <-time.After(time.Duration(sec)):
			EdgeServers.RWLock.Lock()
			now := time.Now()
			for key, svr := range EdgeServers.Svrs {
				if now.Sub(svr.Infos.LastUpdate) > time.Second*(time.Duration(dura)) {
					delete(EdgeServers.Svrs, key)
				}
			}
			EdgeServers.RWLock.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
