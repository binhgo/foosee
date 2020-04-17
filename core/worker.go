package core

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Task ...
type Task = func()

// AppWorker ...
type AppWorker struct {
	Task   Task
	delay  int
	period int
}

// SetTask ..
func (worker *AppWorker) SetTask(fn Task) *AppWorker {
	worker.Task = fn
	return worker
}

// SetDelay ...
func (worker *AppWorker) SetDelay(seconds int) *AppWorker {
	worker.delay = seconds
	return worker
}

// SetRepeatPeriod ...
func (worker *AppWorker) SetRepeatPeriod(seconds int) *AppWorker {
	worker.period = seconds
	return worker
}

// Execute ...
func (worker *AppWorker) Execute() {
	// delay
	time.Sleep(time.Duration(worker.delay) * time.Second)

	// run first time
	worker.Task()
	tick := time.NewTicker(time.Second * time.Duration(worker.period))
	go worker.scheduler(tick)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	tick.Stop()
	os.Exit(0)
}

func (worker *AppWorker) scheduler(tick *time.Ticker) {
	for range tick.C {
		worker.Task()
	}
}
