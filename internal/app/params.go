package app

import (
	"os"
	"sync"
	"sync/atomic"
)

type Param struct {
	project atomic.Value
	stage   atomic.Value
}

func (a *Param) init() {
	a.project.Store("")
	a.stage.Store("")
}

func (a *Param) GetHostname() string {
	hostName, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostName
}

func (a *Param) GetProject() string {
	return a.project.Load().(string)
}

func (a *Param) SetProject(p string) {
	a.project.Store(p)
}

func (a *Param) GetStage() string {
	return a.stage.Load().(string)
}

func (a *Param) SetStage(s string) {
	a.stage.Store(s)
}

var instanceParam *Param
var onceAp sync.Once

// GetParam Param 싱글톤을 반환한다.
func GetParam() *Param {
	onceAp.Do(func() {
		instanceParam = &Param{}
		instanceParam.init()
	})
	return instanceParam
}
