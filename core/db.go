package core

import (
	"crypto/tls"
	"net"
	"reflect"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

type DbResponse struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Total     int64       `json:"total,omitempty"`
}

type DbStatusEnum struct {
	Ok       string
	Error    string
	NotFound string
}

var DbStatus = &DbStatusEnum{
	Ok:       "OK",
	Error:    "ERROR",
	NotFound: "NOT_FOUND",
}

// DBModel ...
type DBModel struct {
	ColName        string
	DBName         string
	TemplateObject interface{}
	collection     *mgo.Collection
	db             *mgo.Database
	mSession       *DBSession
}

// DBSession ..
type DBSession struct {
	session  *mgo.Session
	database string
}

// GetMGOSession : get mgo session
func (s *DBSession) GetMGOSession() *mgo.Session {
	return s.session
}

// Copy : copy dbsession
func (s *DBSession) Copy() *DBSession {
	return &DBSession{
		session: s.session.Copy(),
	}
}

// Valid : check if session is active
func (s *DBSession) Valid() bool {
	return s.session.Ping() == nil
}

// Clone : clone dbsession
func (s *DBSession) Clone() *DBSession {
	return &DBSession{
		session: s.session.Clone(),
	}
}

// Close : close db session
func (s *DBSession) Close() {
	s.session.Close()
}

type Error struct {
	Type    string
	Message string
	Data    interface{}
}

func (e *Error) Error() string {
	return e.Type + " : " + e.Message
}

// DBConfiguration ...
type DBConfiguration struct {
	Address            []string
	Ssl                bool
	Username           string
	Password           string
	AuthDB             string
	ReplicaSetName     string
	SecondaryPreferred bool
}

// DBClient ..
type DBClient struct {
	Name        string
	Config      DBConfiguration
	onConnected OnConnectedHandler
}

// OnConnectedHandler ...
type OnConnectedHandler = func(session *DBSession) error

// OnConnected ...
func (client *DBClient) OnConnected(fn OnConnectedHandler) {
	client.onConnected = fn
}

// Connect ...
func (client *DBClient) Connect() error {
	dialInfo := mgo.DialInfo{
		Addrs:          client.Config.Address,
		Username:       client.Config.Username,
		Password:       client.Config.Password,
		ReplicaSetName: client.Config.ReplicaSetName,
		PoolLimit:      200,
		MinPoolSize:    3,
		MaxIdleTimeMS:  600000,
	}

	if client.Config.AuthDB != "" {
		dialInfo.Database = client.Config.AuthDB
	}

	if client.Config.Ssl {
		tlsConfig := &tls.Config{}
		tlsConfig.InsecureSkipVerify = true
		dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
			return conn, err
		}
	}
	session, err := mgo.DialWithInfo(&dialInfo)
	if client.Config.SecondaryPreferred {
		session.SetMode(mgo.SecondaryPreferred, true)
	}

	if err != nil {
		return err
	}

	err = client.onConnected(&DBSession{session: session, database: client.Config.AuthDB})
	return err
}

// convertToObject convert bson to object
func (m *DBModel) convertToObject(b bson.M) (interface{}, error) {
	obj := m.NewObject()

	if b == nil {
		return obj, nil
	}

	bytes, err := bson.Marshal(b)
	if err != nil {
		return nil, err
	}

	bson.Unmarshal(bytes, obj)
	return obj, nil
}

// convertToBson Go object to map (to get / query)
func (m *DBModel) convertToBson(ent interface{}) (bson.M, error) {
	if ent == nil {
		return bson.M{}, nil
	}

	sel, err := bson.Marshal(ent)
	if err != nil {
		return nil, err
	}

	obj := bson.M{}
	bson.Unmarshal(sel, &obj)

	return obj, nil
}

// Init ...
func (m *DBModel) Init(s *DBSession) error {

	if len(m.DBName) == 0 || len(m.ColName) == 0 {
		return &Error{Type: "INVALID_INPUT", Message: "Require valid DB name and collection name."}
	}
	dbName := s.database
	if s.database == "" || s.database == "admin" {
		dbName = m.DBName
	}
	m.db = s.session.DB(dbName)
	m.collection = m.db.C(m.ColName)
	m.mSession = s
	return nil
}

// NewObject return new object with same type of TemplateObject
func (m *DBModel) NewObject() interface{} {
	t := reflect.TypeOf(m.TemplateObject)
	v := reflect.New(t)
	return v.Interface()
}

// NewList return new object with same type of TemplateObject
func (m *DBModel) NewList(limit int) interface{} {
	t := reflect.TypeOf(m.TemplateObject)
	return reflect.MakeSlice(reflect.SliceOf(t), 0, limit).Interface()
}

// GetFreshSession ...
func (m *DBModel) GetFreshSession() *DBSession {
	return m.mSession.Copy()
}

// GetColWith ...
func (m *DBModel) GetColWith(s *DBSession) (*mgo.Collection, error) {
	if m.collection == nil {
		m.collection = m.db.C(m.ColName)
	}
	return m.collection.With(s.GetMGOSession()), nil
}

// Create insert one object into DB
func (m *DBModel) Create(entity interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:  DbStatus.Error,
			Message: "DB error: " + err.Error(),
		}
	}

	// convert to bson
	obj, err := m.convertToBson(entity)
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "MAP_OBJECT_FAILED",
		}
	}

	// init time
	if obj["created_time"] == nil {
		obj["created_time"] = time.Now()
		obj["last_updated_time"] = obj["created_time"]
	} else {
		obj["last_updated_time"] = time.Now()
	}

	// insert
	err = col.Insert(obj)
	if err != nil {
		return &DbResponse{
			Status:  DbStatus.Error,
			Message: "DB Error: " + err.Error(),
		}
	}

	entity, _ = m.convertToObject(obj)

	list := m.NewList(1)
	listValue := reflect.Append(reflect.ValueOf(list),
		reflect.Indirect(reflect.ValueOf(entity)))

	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Create " + m.ColName + " successfully.",
		Data:    listValue.Interface(),
	}

}

// CreateMany insert many object into db
func (m *DBModel) CreateMany(entityList ...interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}
	objs := []bson.M{}
	ints := []interface{}{}

	if len(entityList) == 1 {
		rt := reflect.TypeOf(entityList[0])
		switch rt.Kind() {
		case reflect.Slice:
			entityList = entityList[0].([]interface{})
		case reflect.Array:
			entityList = entityList[0].([]interface{})
		}
	}
	for _, ent := range entityList {
		obj, err := m.convertToBson(ent)
		if err != nil {
			return &DbResponse{
				Status:    DbStatus.Error,
				Message:   "DB Error: " + err.Error(),
				ErrorCode: "MAP_OBJECT_FAILED",
			}
		}
		if obj["created_time"] == nil {
			obj["created_time"] = time.Now()
			obj["last_updated_time"] = obj["created_time"]
		} else {
			obj["last_updated_time"] = time.Now()
		}
		objs = append(objs, obj)
		ints = append(ints, obj)
	}

	err = col.Insert(ints...)
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "CREATE_FAILED",
		}
	}
	list := m.NewList(len(entityList))
	listValue := reflect.ValueOf(list)
	for _, obj := range objs {
		entity, _ := m.convertToObject(obj)
		listValue = reflect.Append(listValue, reflect.Indirect(reflect.ValueOf(entity)))
	}

	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Create " + m.ColName + "(s) successfully.",
		Data:    listValue.Interface(),
	}
}

// Query Get all object in DB
func (m *DBModel) Query(query interface{}, offset int, limit int, reverse bool) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	q := col.Find(query)
	if limit == 0 {
		limit = 1000
	}
	if limit > 0 {
		q.Limit(limit)
	}
	if offset > 0 {
		q.Skip(offset)
	}
	if reverse {
		q.Sort("-_id")
	}

	list := m.NewList(limit)
	err = q.All(&list)

	if err != nil || reflect.ValueOf(list).Len() == 0 {
		return &DbResponse{
			Status:    DbStatus.NotFound,
			Message:   "Not found any matched " + m.ColName + ".",
			ErrorCode: "NOT_FOUND",
		}
	}
	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Query " + m.ColName + " successfully.",
		Data:    list,
	}
}

// QueryS Get all object in DB with orderby clause
func (m *DBModel) QueryS(query interface{}, offset int, limit int, sortStr string) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	q := col.Find(query)
	if limit == 0 {
		limit = 1000
	}
	if limit > 0 {
		q.Limit(limit)
	}
	if offset > 0 {
		q.Skip(offset)
	}
	if sortStr != "" {
		q.Sort(sortStr)
	}

	list := m.NewList(limit)
	err = q.All(&list)

	if err != nil || reflect.ValueOf(list).Len() == 0 {
		return &DbResponse{
			Status:    DbStatus.NotFound,
			Message:   "Not found any matched " + m.ColName + ".",
			ErrorCode: "NOT_FOUND",
		}
	}
	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Query " + m.ColName + " successfully.",
		Data:    list,
	}
}

// Update Update all matched item
func (m *DBModel) Update(query interface{}, updater interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	obj, err := m.convertToBson(updater)
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "MAP_OBJECT_FAILED",
		}
	}
	obj["last_updated_time"] = time.Now()

	info, err := col.UpdateAll(query, bson.M{
		"$set": obj,
	})
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "Update error: " + err.Error(),
			ErrorCode: "UPDATE_FAILED",
		}
	}

	if info.Matched == 0 {
		return &DbResponse{
			Status:  DbStatus.Ok,
			Message: "Not found any " + m.ColName + ".",
		}
	}

	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Update " + m.ColName + " successfully.",
	}
}

// QueryOne ...
func (m *DBModel) QueryOne(query interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	q := col.Find(query)
	q.Limit(1)

	list := m.NewList(1)
	err = q.All(&list)
	if err != nil || reflect.ValueOf(list).Len() == 0 {
		return &DbResponse{
			Status:    DbStatus.NotFound,
			Message:   "Not found any matched " + m.ColName + ".",
			ErrorCode: "NOT_FOUND",
		}
	}
	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Query " + m.ColName + " successfully.",
		Data:    list,
	}
}

// UpdateOne Update one matched object.
func (m *DBModel) UpdateOne(query interface{}, updater interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	bUpdater, err := m.convertToBson(updater)
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "MAP_OBJECT_FAILED",
		}
	}
	bUpdater["last_updated_time"] = time.Now()

	change := mgo.Change{
		Update:    bson.M{"$set": bUpdater},
		ReturnNew: true,
	}
	obj := m.NewObject()
	q := col.Find(query)
	q.Limit(1)
	info, err := q.Apply(change, obj)
	list := m.NewList(1)
	listValue := reflect.Append(reflect.ValueOf(list),
		reflect.Indirect(reflect.ValueOf(obj)))

	if (err != nil && err.Error() == "not found") || (info != nil && info.Matched == 0) {
		return &DbResponse{
			Status:  DbStatus.NotFound,
			Message: "Not found any " + m.ColName + ".",
		}
	}

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "Update error: " + err.Error(),
			ErrorCode: "UPDATE_FAILED",
		}
	}

	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Update one " + m.ColName + " successfully.",
		Data:    listValue.Interface(),
	}
}

// UpsertOne Update one matched object, if notfound, create new document
func (m *DBModel) UpsertOne(query interface{}, updater interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	bUpdater, err := m.convertToBson(updater)
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "MAP_OBJECT_FAILED",
		}
	}
	now := time.Now()
	bUpdater["last_updated_time"] = now

	change := mgo.Change{
		Update: bson.M{
			"$set": bUpdater,
			"$setOnInsert": bson.M{
				"created_time": now,
			},
		},
		ReturnNew: true,
		Upsert:    true,
	}

	obj := m.NewObject()
	_, err = col.Find(query).Limit(1).Apply(change, obj)
	list := m.NewList(1)
	listValue := reflect.Append(reflect.ValueOf(list),
		reflect.Indirect(reflect.ValueOf(obj)))
	if err != nil {
		if err.Error() == "not found" {
			return &DbResponse{
				Status:  DbStatus.NotFound,
				Message: "Not found any " + m.ColName + ".",
			}
		}
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "UPSERT_ONE_ERROR",
		}
	}
	return &DbResponse{
		Status:  DbStatus.Ok,
		Data:    listValue.Interface(),
		Message: "Upsert one " + m.ColName + " successfully.",
	}
}

// Delete Delete all object which matched with selector
func (m *DBModel) Delete(selector interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	err = col.Remove(selector)
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "Delete error: " + err.Error(),
			ErrorCode: "DELETE_FAILED",
		}
	}
	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Delete " + m.ColName + " successfully.",
	}
}

// Count Count object which matched with query.
func (m *DBModel) Count(query interface{}) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	count, err := col.Find(query).Count()
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "Count error: " + err.Error(),
			ErrorCode: "COUNT_FAILED",
		}
	}

	return &DbResponse{
		Status:  DbStatus.Ok,
		Message: "Count query executed successfully.",
		Total:   int64(count),
	}

}

// IncreOne Increase one field of the document & return new value
func (m *DBModel) IncreOne(query interface{}, fieldName string, value int) *DbResponse {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)
	if err != nil {
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "GET_COLLECTION_FAILED",
		}
	}

	updater := bson.M{}
	updater[fieldName] = value
	change := mgo.Change{
		Update:    bson.M{"$inc": updater},
		ReturnNew: true,
		Upsert:    true,
	}

	obj := m.NewObject()
	_, err = col.Find(query).Limit(1).Apply(change, obj)
	list := m.NewList(1)
	listValue := reflect.Append(reflect.ValueOf(list),
		reflect.Indirect(reflect.ValueOf(obj)))
	if err != nil {
		if err.Error() == "not found" {
			return &DbResponse{
				Status:  DbStatus.NotFound,
				Message: "Not found any " + m.ColName + ".",
			}
		}
		return &DbResponse{
			Status:    DbStatus.Error,
			Message:   "DB Error: " + err.Error(),
			ErrorCode: "INCREMENT_ERROR",
		}
	}
	return &DbResponse{
		Status:  DbStatus.Ok,
		Data:    listValue.Interface(),
		Message: "Increase " + fieldName + " of " + m.ColName + " successfully.",
	}
}

// CreateIndex ...
func (m *DBModel) CreateIndex(index mgo.Index) error {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)
	if err == nil {
		crErr := col.EnsureIndex(index)
		return crErr
	}
	return err
}

// Aggregate ...
func (m *DBModel) Aggregate(pipeline interface{}, result interface{}) error {
	s := m.GetFreshSession()
	defer s.Close()
	col, err := m.GetColWith(s)

	if err != nil {
		return err
	}
	q := col.Pipe(pipeline)
	return q.All(result)
}
