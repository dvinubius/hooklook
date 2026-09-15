package main

import (
	"net/http"
	"time"
)

type reqMethod string

const (
	GET     reqMethod = "GET"
	POST    reqMethod = "POST"
	PUT     reqMethod = "PUT"
	PATCH   reqMethod = "PATCH"
	DELETE  reqMethod = "DELETE"
	OPTIONS reqMethod = "OPTIONS"
	HEAD    reqMethod = "HEAD"
	TRACE   reqMethod = "TRACE"
	CONNECT reqMethod = "CONNECT"
)

type ParsedRequest struct {
	Id          string    `json:"id"`
	Method      reqMethod `json:"method"`
	Path        string    `json:"path"`
	ReceiptTime time.Time `json:"receiptTime"`
}

func parseRequest(req *http.Request, path string) ParsedRequest {
	return ParsedRequest{
		Method:      reqMethod(req.Method),
		Path:        path,
		ReceiptTime: time.Now().UTC().Truncate(time.Second),
	}
}
