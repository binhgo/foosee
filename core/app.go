package core

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// App ..
type App struct {
	Name             string
	Server           *Server
	DBList           []*DBClient
	WorkerList       []*Worker
	onAllDBConnected Task
	launched         bool
	hostname         string
}

// NewApp Wrap application
func NewApp(name string) *App {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "undefined"
	}

	app := &App{
		Name:       name,
		Server:     &Server{},
		DBList:     []*DBClient{},
		WorkerList: []*Worker{},
		launched:   false,
		hostname:   hostname,
	}
	return app
}

// SetupDBClient ...
func (app *App) SetupDBClient(config DBConfiguration) *DBClient {
	var db = &DBClient{Config: config}
	app.DBList = append(app.DBList, db)
	return db
}

// SetupDBClient ...
func (app *App) OnAllDBConnected(task Task) {
	app.onAllDBConnected = task
}

// SetupAPIServer ...
func (app *App) SetupAPIServer() (*Server, error) {

	var sv *Server
	sv = NewServer(-10)
	if sv == nil {
		return nil, errors.New("server type " + " is invalid (HTTP/THRIFT)")
	}

	app.Server = sv
	return sv, nil
}

// SetupWorker ...
func (app *App) SetupWorker() *Worker {
	var worker = &Worker{}
	app.WorkerList = append(app.WorkerList, worker)
	return worker
}

// Launch Launch app
func (app *App) Launch() error {

	if app.launched {
		return nil
	}

	app.launched = true

	name := app.Name + " / " + app.hostname
	fmt.Println("[ App " + name + " ] Launching ...")
	var wg = sync.WaitGroup{}

	// start connect to DB
	for _, db := range app.DBList {
		err := db.Connect()
		if err != nil {
			fmt.Println("Connect DB error " + err.Error())
			return err
		}
	}

	fmt.Println("[ App " + name + " ] DBs connected.")

	if app.onAllDBConnected != nil {
		app.onAllDBConnected()
		fmt.Println("[ App " + name + " ] On-all-DBs-connected handler executed.")
	}

	// start servers
	go app.Server.Start()
	fmt.Println("[ App " + name + " ] Servers started.")

	// start workers
	for _, wk := range app.WorkerList {
		wg.Add(1)
		go wk.Execute()
	}
	fmt.Println("[ App " + name + " ] Workers started.")
	fmt.Println("[ App " + name + " ] Totally launched!")
	wg.Wait()

	return nil
}
