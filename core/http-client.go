package core

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/globalsign/mgo"
)

type APIResponse struct {
	Status    string            `json:"status"`
	Data      interface{}       `json:"data,omitempty"`
	Message   string            `json:"message"`
	ErrorCode string            `json:"errorCode,omitempty"`
	Total     int64             `json:"total,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// StatusEnum ...
type StatusEnum struct {
	Ok           string
	Error        string
	Invalid      string
	NotFound     string
	Forbidden    string
	Existed      string
	Unauthorized string
}

// APIStatus Published enum
var APIStatus = &StatusEnum{
	Ok:           "OK",
	Error:        "ERROR",
	Invalid:      "INVALID",
	NotFound:     "NOT_FOUND",
	Forbidden:    "FORBIDDEN",
	Existed:      "EXISTED",
	Unauthorized: "UNAUTHORIZED",
}

// APIClientConfiguration
type APIClientConfiguration struct {
	Address       string
	Protocol      string
	Timeout       time.Duration
	MaxRetry      int
	WaitToRetry   time.Duration
	LoggingCol    string
	MaxConnection int
}

// RestClient :
type RestClient struct {
	BaseURL   *url.URL
	UserAgent string

	// private
	httpClient   *http.Client
	logName      string
	logModel     *DBModel
	maxRetryTime int
	waitTime     time.Duration // milisecond
	timeOut      time.Duration // milisecond

	debug bool
}

// RequestLogEntry ...
type RequestLogEntry struct {
	Status      string             `json:"status,omitempty" bson:"status,omitempty"`
	ReqURL      string             `json:"reqUrl,omitempty" bson:"req_url,omitempty"`
	ReqMethod   string             `json:"reqMethod,omitempty" bson:"req_method,omitempty"`
	ReqHeader   *map[string]string `json:"reqHeader,omitempty" bson:"req_header,omitempty"`
	ReqFormData *map[string]string `json:"reqFormData,omitempty" bson:"req_form_data,omitempty"`
	ReqBody     *interface{}       `json:"reqBody,omitempty" bson:"req_body,omitempty"`

	TotalTime  int64         `json:"totalTime,omitempty" bson:"total_time,omitempty"`
	RetryCount int           `json:"retryCount,omitempty" bson:"retry_count,omitempty"`
	Results    []*CallResult `json:"results,omitempty" bson:"results,omitempty"`
	ErrorLog   *string       `json:"errorLog,omitempty" bson:"error_log,omitempty"`
	Keys       *[]string     `json:"keys,omitempty" bson:"keys,omitempty"`
	Date       *time.Time    `bson:"date,omitempty" json:"date,omitempty"`
}

// CallResult ...
type CallResult struct {
	RespCode     int                 `json:"respCode,omitempty" bson:"resp_code,omitempty"`
	RespHeader   map[string][]string `json:"respHeader,omitempty" bson:"resp_header,omitempty"`
	RespBody     *string             `json:"respBody,omitempty" bson:"resp_body,omitempty"`
	ResponseTime int64               `json:"responseTime,omitempty" bson:"response_time,omitempty"`
	ErrorLog     *string             `json:"errorLog,omitempty" bson:"error_log,omitempty"`
}

// RestResult :
type RestResult struct {
	Body    string `json:"body,omitempty" bson:"body,omitempty"`
	Content []byte `json:"content,omitempty" bson:"content,omitempty"`
	Code    int    `json:"code,omitempty" bson:"code,omitempty"`
}

// HTTPMethod ...
type HTTPMethod string

// HTTPMethodEnum ...
type HTTPMethodEnum struct {
	Get    HTTPMethod
	Post   HTTPMethod
	Put    HTTPMethod
	Head   HTTPMethod
	Delete HTTPMethod
	Option HTTPMethod
}

// HTTPMethods Supported HTTP Method
var HTTPMethods = &HTTPMethodEnum{
	Get:    "GET",
	Post:   "POST",
	Put:    "PUT",
	Head:   "HEAD",
	Delete: "DELETE",
	Option: "OPTION",
}

// NewHTTPClient
func NewHTTPClient(config *APIClientConfiguration) *RestClient {
	return NewRESTClient(config.Address, config.LoggingCol,
		config.Timeout,
		config.MaxRetry,
		config.WaitToRetry)
}

// NewRESTClient : New instance of restClient
func NewRESTClient(baseURL string, logName string, timeout time.Duration, maxRetryTime int, waitTime time.Duration) *RestClient {

	var restCl RestClient

	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "http://" + baseURL
	}

	u, err := url.Parse(baseURL)
	if err == nil {
		restCl.BaseURL = u
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	restCl.httpClient = &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	restCl.SetMaxRetryTime(maxRetryTime)
	restCl.SetWaitTime(waitTime)
	restCl.SetTimeout(timeout)
	restCl.debug = false
	if logName != "" {
		restCl.SetLoggerName(logName)
	}

	return &restCl
}

// addParam
func addParams(baseURL string, params map[string]string) string {
	baseURL += "?"
	p := url.Values{}
	for key, value := range params {
		p.Add(key, value)
	}
	return baseURL + p.Encode()
}

func (entry *RequestLogEntry) addResult(rs *CallResult) {
	entry.Results = append(entry.Results, rs)
}

// SetLoggerName :
func (c *RestClient) SetLoggerName(loggerName string) {
	c.logName = loggerName
}

// SetDBLog :
func (c *RestClient) SetDBLog(dbName string, session *DBSession) {
	colName := "undefined"
	if c.logName != "" {
		colName = c.logName
	}

	model := DBModel{
		ColName: colName,
		DBName:  dbName,
	}
	err := model.Init(session)
	if err == nil {
		index := mgo.Index{
			Key:         []string{"created_time"},
			Background:  true,
			Sparse:      true,
			ExpireAfter: time.Duration(1814400) * time.Second, // 21 day
		}
		model.CreateIndex(index)

		keyIndex := mgo.Index{
			Key:        []string{"keys"},
			Background: true,
			Sparse:     true,
		}
		model.CreateIndex(keyIndex)
	}

	c.logModel = &model
}

func (c *RestClient) SetDebug(val bool) {
	c.debug = val
}

// SetTimeout :
func (c *RestClient) SetTimeout(timeout time.Duration) {
	c.timeOut = timeout
	c.httpClient.Timeout = timeout
}

// SetWaitTime :
func (c *RestClient) SetWaitTime(waitTime time.Duration) {
	c.waitTime = waitTime
}

// SetMaxRetryTime :
func (c *RestClient) SetMaxRetryTime(maxRetryTime int) {
	c.maxRetryTime = maxRetryTime
}

func (c *RestClient) initRequest(method HTTPMethod, headers map[string]string, params map[string]string, body interface{}, path string) (*http.Request, error) {

	u := c.BaseURL.String()
	if path != "" {
		if strings.HasSuffix(u, "/") || strings.HasPrefix(path, "/") {
			u = u + path
		} else {
			u = u + "/" + path
		}
	}
	urlStr := addParams(u, params)

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	var err error

	var req *http.Request
	if method == HTTPMethods.Post && headers != nil && headers["Content-Type"] == "application/x-www-form-urlencoded" && params != nil && len(params) > 0 {
		data := url.Values{}
		for key, val := range params {
			data.Set(key, val)
		}
		req, err = http.NewRequest(string(method), urlStr, strings.NewReader(data.Encode()))
	} else {
		req, err = http.NewRequest(string(method), urlStr, buf)
	}

	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	userAgent := "Go-RESTClient/1.0"
	hostname, err := os.Hostname()
	if err == nil {
		userAgent += " " + hostname + "/" + os.Getenv("env")
	}

	req.Header.Set("User-Agent", userAgent)

	// set header
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// MakeHTTPRequest :
func (c *RestClient) MakeHTTPRequest(method HTTPMethod, headers map[string]string, params map[string]string, body interface{}, path string) (*RestResult, error) {
	return c.MakeHTTPRequestWithKey(method, headers, params, body, path, nil)
}

func (c *RestClient) writeLog(logEntry *RequestLogEntry) {

	if c.debug {
		fmt.Println(" +++ Writing log ...")
	}
	go c.logModel.Create(logEntry)

}

// MakeHTTPRequestWithKey
func (c *RestClient) MakeHTTPRequestWithKey(method HTTPMethod, headers map[string]string, params map[string]string, body interface{}, path string, keys *[]string) (*RestResult, error) {

	date := time.Now()
	// init log
	logEntry := &RequestLogEntry{
		ReqURL:      c.BaseURL.String() + path,
		ReqMethod:   string(method),
		ReqFormData: &params,
		ReqHeader:   &headers,
		ReqBody:     &body,
		Keys:        keys,
		Date:        &date,
	}

	if c.logModel != nil {
		defer c.writeLog(logEntry)
	}

	if c.debug {
		fmt.Println(" +++ Try to init request ...")
	}

	canRetryCount := c.maxRetryTime

	tstart := time.Now().UnixNano() / 1e6

	for canRetryCount >= 0 {

		req, reqErr := c.initRequest(method, headers, params, body, path)

		if c.debug {
			fmt.Println(" +++ Init request successfully.")
		}

		if reqErr != nil {
			msg := reqErr.Error()
			logEntry.ErrorLog = &msg
			return nil, reqErr
		}
		// start time
		startCallTime := time.Now().UnixNano() / 1e6
		if c.debug {
			fmt.Println("+++ Let call: " + logEntry.ReqMethod + " " + logEntry.ReqURL)
		}

		// add call result
		callRs := &CallResult{}

		// do request
		resp, err := c.httpClient.Do(req)
		if c.debug {
			fmt.Println("+++ HTTP call ended!")
		}

		// make request successful
		if err == nil {
			restResult, err := c.readBody(resp, callRs, logEntry, canRetryCount, startCallTime, tstart)
			if restResult != nil {
				logEntry.Status = "SUCCESS"
				return restResult, err
			}
		} else {
			if c.debug {
				fmt.Println("HTTP Error: " + err.Error())
			}
			msg := err.Error()
			callRs.ErrorLog = &msg
		}

		tend := time.Now().UnixNano() / 1e6
		callRs.ResponseTime = tend - startCallTime

		canRetryCount--

		if canRetryCount >= 0 {
			time.Sleep(c.waitTime)
			if c.debug {
				fmt.Println("Comeback from sleep ...")
			}
		}

		if c.debug {
			fmt.Println("Count down ...")
		}
		if canRetryCount >= 0 {
			logEntry.RetryCount = c.maxRetryTime - canRetryCount
		}
		logEntry.addResult(callRs)
		if c.debug {
			fmt.Println("Try to exit loop ...")
		}
	}

	if c.debug {
		fmt.Println("Exit retry loop.")
	}

	tend := time.Now().UnixNano() / 1e6
	logEntry.TotalTime = tend - tstart
	logEntry.Status = "FAILED"
	return nil, errors.New("fail to call endpoint API " + logEntry.ReqURL)
}

func (c *RestClient) readBody(resp *http.Response, callRs *CallResult, logEntry *RequestLogEntry, canRetryCount int, startCallTime int64, tstart int64) (*RestResult, error) {
	defer resp.Body.Close()
	v, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := err.Error()
		callRs.ErrorLog = &msg
		return nil, err
	}

	if c.debug {
		fmt.Println("+++ IO read ended!")
	}
	restResult := RestResult{
		Code:    resp.StatusCode,
		Body:    string(v),
		Content: v,
	}

	encoding := resp.Header.Get("Content-Encoding")
	if encoding == "gzip" {
		if c.debug {
			fmt.Println("+++ Start to gunzip")
		}
		gr, _ := gzip.NewReader(bytes.NewBuffer(restResult.Content))
		data, err := ioutil.ReadAll(gr)
		gr.Close()
		if err != nil {
			return nil, err
		}
		if c.debug {
			fmt.Println("+++ gunzip successfully")
		}
		restResult.Content = data
		restResult.Body = string(data)
	}

	// set call result
	callRs.RespCode = restResult.Code
	callRs.RespBody = &restResult.Body
	if resp.Header != nil {
		h := (map[string][]string)(resp.Header)
		if h != nil {
			callRs.RespHeader = map[string][]string{}
			for k, v := range h {
				if strings.HasPrefix(k, "X-") {
					callRs.RespHeader[k] = v
				}
			}
		}
	}

	if c.debug {
		fmt.Println("+++ Read data end, http code: " + string(resp.StatusCode))
	}
	if (resp.StatusCode >= 200 && resp.StatusCode < 300) || (resp.StatusCode >= 400 && resp.StatusCode < 500) {
		// add log
		tend := time.Now().UnixNano() / 1e6
		callRs.ResponseTime = tend - startCallTime
		logEntry.TotalTime = tend - tstart
		if canRetryCount >= 0 {
			logEntry.RetryCount = c.maxRetryTime - canRetryCount
		}
		// sample
		logEntry.addResult(callRs)
		// return
		return &restResult, err
	}
	return nil, nil
}

// MakeRequest ...
func (c *RestClient) MakeRequest(req OutboundAPIRequest) *APIResponse {
	var data interface{}
	var reqMethod = req.GetMethod()
	var method HTTPMethod

	switch reqMethod.Value {
	case "GET":
		method = HTTPMethods.Get
	case "PUT":
		method = HTTPMethods.Put
		req.GetContent(&data)
	case "POST":
		method = HTTPMethods.Post
		req.GetContent(&data)
	case "DELETE":
		method = HTTPMethods.Delete
	case "OPTIONS":
		method = HTTPMethods.Option
	}

	if c.debug {
		fmt.Println("Req info: " + reqMethod.Value + " / " + req.GetPath())
		if data != nil {
			fmt.Println("Data not null")
		}
	}

	result, err := c.MakeHTTPRequest(method, req.GetHeaders(), req.GetParams(), data, req.GetPath())

	if err != nil {
		return &APIResponse{
			Status:  APIStatus.Error,
			Message: "HTTP Endpoint Error: " + err.Error(),
		}
	}

	var resp = &APIResponse{}
	err = json.Unmarshal(result.Content, &resp)

	if resp.Status == "" {
		if result.Code >= 500 {
			resp.Status = APIStatus.Error
		} else if result.Code >= 400 {
			if result.Code == 404 {
				resp.Status = APIStatus.NotFound
			} else if result.Code == 403 {
				resp.Status = APIStatus.Forbidden
			} else if result.Code == 401 {
				resp.Status = APIStatus.Unauthorized
			} else {
				resp.Status = APIStatus.Invalid
			}
		} else {
			resp.Status = APIStatus.Ok
		}
	}

	if err != nil {
		return &APIResponse{
			Status:  APIStatus.Error,
			Message: "Response Data Error: " + err.Error(),
			Data:    []string{result.Body},
		}
	}
	return resp
}

// OutboundAPIRequest Request to call other service
type OutboundAPIRequest struct {
	Method  string            `json:"method" bson:"method"`
	Path    string            `json:"path" bson:"path"`
	Params  map[string]string `json:"params,omitempty" bson:"params,omitempty"`
	Headers map[string]string `json:"headers,headers" bson:"headers,omitempty"`
	Content string            `json:"content,omitempty" bson:"content,omitempty"`
}

type MethodValue struct {
	Value string
}

// MethodEnum ...
type MethodEnum struct {
	GET     *MethodValue
	POST    *MethodValue
	PUT     *MethodValue
	DELETE  *MethodValue
	OPTIONS *MethodValue
}

// APIMethod Published enum
var APIMethod = MethodEnum{
	GET:     &MethodValue{Value: "GET"},
	POST:    &MethodValue{Value: "POST"},
	PUT:     &MethodValue{Value: "PUT"},
	DELETE:  &MethodValue{Value: "DELETE"},
	OPTIONS: &MethodValue{Value: "OPTIONS"},
}

func NewOutboundAPIRequest(method string, path string, params map[string]string, content string, headers map[string]string) *OutboundAPIRequest {
	return &OutboundAPIRequest{
		Method:  method,
		Path:    path,
		Params:  params,
		Content: content,
		Headers: headers,
	}
}

// GetPath ..
func (req *OutboundAPIRequest) GetPath() string {
	return req.Path
}

// GetPath ..
func (req *OutboundAPIRequest) GetIP() string {
	return "GetIP() not implemented"
}

// GetMethod ..
func (req *OutboundAPIRequest) GetMethod() *MethodValue {
	var s = req.Method
	switch s {
	case "GET":
		return APIMethod.GET
	case "POST":
		return APIMethod.POST
	case "PUT":
		return APIMethod.PUT
	case "DELETE":
		return APIMethod.DELETE
	}

	return &MethodValue{Value: s}
}

// GetParams ...
func (req *OutboundAPIRequest) GetParams() map[string]string {
	return req.Params
}

// GetContent ...
func (req *OutboundAPIRequest) GetContent(data interface{}) error {
	json.Unmarshal([]byte(req.Content), &data)
	return nil
}

// GetContentText ...
func (req *OutboundAPIRequest) GetContentText() string {
	return req.Content
}

// GetHeaders ...
func (req *OutboundAPIRequest) GetHeaders() map[string]string {
	return req.Headers
}
