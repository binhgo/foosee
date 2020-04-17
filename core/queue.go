package core

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

const None = "NONE"

type ConsumeFn = func(*QueueItem) error

type DBQueueChannel struct {
	name       string
	item       *QueueItem
	isActive   bool
	processing bool
	consumer   ConsumeFn
	queueDB    *DBModel
	consumedDB *DBModel
	lock       *sync.Mutex
}

type QueueItem struct {
	Data            interface{}    `json:"data,omitempty" bson:"data,omitempty"`
	ID              *bson.ObjectId `json:"id,omitempty" bson:"_id,omitempty"`
	ProcessBy       string         `json:"processBy,omitempty" bson:"process_by,omitempty"`
	Version         string         `json:"version,omitempty" bson:"versionF,omitempty"`
	ProcessTimeMS   int            `json:"processTimeMS,omitempty" bson:"process_time_ms,omitempty"`
	CreatedTime     *time.Time     `json:"createdTime,omitempty" bson:"created_time,omitempty"`
	LastUpdatedTime *time.Time     `json:"lastUpdatedTime,omitempty" bson:"last_updated_time,omitempty"`
	LastFail        *time.Time     `json:"lastFail,omitempty" bson:"last_fail,omitempty"`
	Keys            *[]string      `json:"keys,omitempty" bson:"keys,omitempty"`
	Log             *[]string      `json:"log,omitempty" bson:"log,omitempty"`
}

func (c *DBQueueChannel) start() {
	for true {

		// sleep 50ms when free
		if c.item == nil {
			time.Sleep(50 * time.Millisecond)
		} else {
			// the more the item fail, it require wait more time
			// limit max 5 second
			if c.item.LastFail != nil && time.Since(*c.item.LastFail).Seconds() < 3 {
				time.Sleep(time.Duration(len(*c.item.Log)) * time.Second)
			}

			start := time.Now()
			err := c.consumer(c.item)

			if err == nil { // if successfully
				c.queueDB.Delete(&QueueItem{
					ID: c.item.ID,
				})
				t := time.Since(start).Nanoseconds() / 1000000
				c.item.ProcessTimeMS = int(t)
				c.item.ID = nil
				c.consumedDB.Create(c.item)
			} else { // if consuming function return errors
				errStr := c.name + " " + time.Now().Format("2006-01-02T15:04:05+0700") + " " + err.Error()
				if c.item.Log == nil {
					c.item.Log = &[]string{errStr}
				} else {
					l := len(*c.item.Log)
					if l > 4 { // crop if too long
						tmp := (*c.item.Log)[l-4:]
						c.item.Log = &tmp
					}
					log := append(*c.item.Log, errStr)
					c.item.Log = &log
				}

				// update item
				now := time.Now()
				c.queueDB.UpdateOne(&QueueItem{
					ID: c.item.ID,
				}, &QueueItem{
					Log:       c.item.Log,
					ProcessBy: None,
					LastFail:  &now,
				})
			}

			c.processing = false
			c.item = nil
		}
	}
}

func (c *DBQueueChannel) putItem(item *QueueItem) bool {
	c.item = item
	return true
}

// DBQueueConnector ...
type DBQueueConnector struct {
	name     string
	channels []*DBQueueChannel
	queueDB  *DBModel
	version  string
}

func (dbc *DBQueueConnector) pickFreeChannel(start int, limit int) int {
	var quota = limit
	var i = start
	for quota > 0 {
		if dbc.channels[i].item == nil && !dbc.channels[i].processing {
			dbc.channels[i].lock.Lock()
			if dbc.channels[i].item == nil && !dbc.channels[i].processing {
				dbc.channels[i].processing = true
				dbc.channels[i].lock.Unlock()
				return i
			}
			dbc.channels[i].lock.Unlock()
		}
		i = (i + 1) % limit
		quota--
	}
	return -1
}

func (dbc *DBQueueConnector) start() {
	var channelNum = len(dbc.channels)
	var i = 0
	var counter = 0
	for true {

		// pick channel
		picked := -1
		for picked < 0 {
			picked = dbc.pickFreeChannel(i, channelNum)
			if picked >= 0 {
				i = (picked + 1) % channelNum
			} else {
				// if all channels are busy
				time.Sleep(10 * time.Millisecond)
			}
		}

		// pick one item in queue
		resp := dbc.queueDB.UpdateOne(&bson.M{
			"process_by": None,
		}, &bson.M{
			"process_by":       dbc.name + " - " + dbc.channels[picked].name,
			"consumer_version": dbc.version,
		})

		if resp.Status == DbStatus.Ok {
			item := resp.Data.([]*QueueItem)[0]
			dbc.channels[picked].putItem(item)
		} else {
			// if no item found
			dbc.channels[picked].processing = false
			time.Sleep(1 * time.Second)
		}

		// check & clean old item
		counter++
		if counter > 100 {

			// clean old item with different version if over 15 minutes
			oldTime := time.Now().Add(-(15 * time.Minute))
			dbc.queueDB.Update(&bson.M{
				"process_by": bson.M{
					"$ne": None,
				},
				"consumer_version": bson.M{
					"$ne": dbc.version,
				},
				"last_updated_time": bson.M{
					"$lt": oldTime,
				},
			}, &bson.M{
				"process_by":       None,
				"consumer_version": None,
			})

			// don't care version if over 1 hour
			oldTime = oldTime.Add(-(45 * time.Minute))
			dbc.queueDB.Update(&bson.M{
				"process_by": bson.M{
					"$ne": None,
				},
				"last_updated_time": bson.M{
					"$lt": oldTime,
				},
			}, &bson.M{
				"process_by":       None,
				"consumer_version": None,
			})

			counter = 0
		}

	}
}

// DBQueue2 ...
type DBQueue2 struct {
	ColName    string
	queueDB    *DBModel
	consumedDB *DBModel
	ready      bool
	channels   []*DBQueueChannel
	connector  *DBQueueConnector
	hostname   string
}

// Init ...
func (dbq *DBQueue2) Init(mSession *DBSession, dbName string) {

	// setup main queue
	dbq.InitWithExpiredTime(mSession, dbName, time.Duration(7*24)*time.Hour)
}

// Init ...
func (dbq *DBQueue2) InitWithExpiredTime(mSession *DBSession, dbName string, expiredTime time.Duration) {

	// setup main queue
	dbq.queueDB = &DBModel{
		ColName:        dbq.ColName,
		DBName:         dbName,
		TemplateObject: &QueueItem{},
	}
	dbq.queueDB.Init(mSession)
	dbq.queueDB.CreateIndex(mgo.Index{
		Key:        []string{"process_by"},
		Background: true,
	})
	dbq.queueDB.CreateIndex(mgo.Index{
		Key:        []string{"keys"},
		Background: true,
	})

	// setup consumed (for history)
	dbq.consumedDB = &DBModel{
		ColName:        dbq.ColName + "_consumed",
		DBName:         dbName,
		TemplateObject: &QueueItem{},
	}

	dbq.consumedDB.Init(mSession)
	dbq.consumedDB.CreateIndex(mgo.Index{
		Key:         []string{"last_updated_time"},
		Background:  true,
		ExpireAfter: expiredTime,
	})
	dbq.consumedDB.CreateIndex(mgo.Index{
		Key:        []string{"keys"},
		Background: true,
	})

	dbq.ready = true
}

// StartConsumer ...
func (dbq *DBQueue2) StartConsumer(consumer ConsumeFn, channelNum int) {
	if dbq.ready == false {
		panic(Error{Type: "NOT_INITED", Message: "Require to init database before using queue."})
	}
	if channelNum <= 0 {
		channelNum = 1
	} else if channelNum > 50 {
		channelNum = 50
	}

	dbq.channels = []*DBQueueChannel{}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "undefined"
	}
	dbq.hostname = hostname
	for i := 0; i < channelNum; i++ {
		c := &DBQueueChannel{
			name:       hostname + "/" + strconv.Itoa(i+1),
			consumer:   consumer,
			isActive:   true,
			processing: false,
			queueDB:    dbq.queueDB,
			consumedDB: dbq.consumedDB,
			lock:       &sync.Mutex{},
		}
		dbq.channels = append(dbq.channels, c)
		go c.start()
	}
	go dbq.startConnectors()

}

// startConnector start the job that query item from DB and deliver to channel
func (dbq *DBQueue2) startConnectors() {
	// wait some time for all channels inited
	time.Sleep(3 * time.Second)

	dbq.connector = &DBQueueConnector{
		name:     dbq.hostname + "/connector",
		queueDB:  dbq.queueDB,
		channels: dbq.channels,
	}

	dbq.connector.version = os.Getenv("version")
	if dbq.connector.version == "" {
		dbq.connector.version = strconv.Itoa((1000000 + rand.Int()) % 999999)
	}
	go dbq.connector.start()
}

func (dbq *DBQueue2) Push(data interface{}) error {
	return dbq.PushWithKey(data, nil)
}

func (dbq *DBQueue2) PushWithKey(data interface{}, keys *[]string) error {
	if dbq.ready == false {
		panic(Error{Type: "NOT_INITED", Message: "Require to init database before using queue."})
	}

	var item = QueueItem{
		Data:      data,
		Keys:      keys,
		ProcessBy: None,
	}

	resp := dbq.queueDB.Create(&item)
	if resp.Status == DbStatus.Ok {
		return nil
	}

	fmt.Println("Push to queue" + resp.Status)
	return &Error{Type: resp.ErrorCode, Message: resp.Message}
}
