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
type Worker struct {
	Task   Task
	delay  int
	period int
}

// SetTask ..
func (worker *Worker) SetTask(fn Task) *Worker {
	worker.Task = fn
	return worker
}

// SetDelay ...
func (worker *Worker) SetDelay(seconds int) *Worker {
	worker.delay = seconds
	return worker
}

// SetRepeatPeriod ...
func (worker *Worker) SetRepeatPeriod(seconds int) *Worker {
	worker.period = seconds
	return worker
}

// Execute ...
func (worker *Worker) Execute() {
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

func (worker *Worker) scheduler(tick *time.Ticker) {
	for range tick.C {
		worker.Task()
	}
}
