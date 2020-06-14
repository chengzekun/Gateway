package svrpool

import "sync"

type Scheduler interface {
	Select() (Invoker, error)
}

var (
	SchedulerPool = &sync.Map{}
)
