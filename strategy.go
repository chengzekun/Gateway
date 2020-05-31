package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"
)

var (
	servers []Server
	rwLock  sync.RWMutex
)

func updateServInfos(ctx context.Context, t int64) {
	for {
		select {
		case <-time.After(time.Duration(t) * time.Millisecond):
			rwLock.Lock()
			update()
			rwLock.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func update() {
	EdgeServers.RWLock.RLock()
	defer EdgeServers.RWLock.RUnlock()

	for _, svr := range EdgeServers.Svrs {
		servers = append(servers, svr.Clone())
	}
}

func BalanceWeightRandom() (*Server, error) {
	update()
	rwLock.RLock()
	defer rwLock.RUnlock()
	// 计算权重的总和
	var wSum int32 = 0
	for _, svr := range servers {
		wSum += svr.Infos.Weight
	}
	// 产生一个[0, wSum] 的随机数
	rand.Seed(time.Now().Unix())
	randWt := rand.Int31n(wSum)
	for _, svr := range servers {
		randWt -= svr.Infos.Weight
		if randWt < 0 {
			log.Printf("Choose the server : %+v", svr)
			return &svr, nil
		}
	}

	return nil, errors.New("can not get one instance")
}
