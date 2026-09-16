package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"x-access-token":      true,
}

const redactedValue = "[REDACTED]"

var ErrRequestBodyTooLarge = errors.New("request body too large")

type HeaderMap map[string][]string

type ParsedRequest struct {
	Id          string    `json:"id"`
	Method      reqMethod `json:"method"`
	Path        string    `json:"path"`
	ReceiptTime time.Time `json:"receiptTime"`
	RawQuery    string    `json:"rawQuery"`
	Headers     HeaderMap `json:"headers"`
	ContentType string    `json:"contentType"`
	RawBody     []byte    `json:"rawBody"`
	BodySizeKiB int       `json:"bodySizeKiB"`
}

type SummarizedRequest struct {
	Id          string    `json:"id"`
	Method      reqMethod `json:"method"`
	Path        string    `json:"path"`
	ReceiptTime time.Time `json:"receiptTime"`
}

func getRedactedHeaders(req *http.Request) HeaderMap {
	headers := HeaderMap{}
	for name, values := range req.Header {
		if sensitiveHeaders[strings.ToLower(name)] {
			headers[name] = make([]string, len(values))
			for i := range values {
				headers[name][i] = redactedValue
			}
			continue
		}

		headers[name] = append([]string(nil), values...)
	}
	return headers
}

func getContentType(headers HeaderMap) string {
	for name, values := range headers {
		if strings.EqualFold(name, "Content-Type") && len(values) > 0 {
			return values[0]
		}
	}

	return ""
}

func readBody(req *http.Request, maxBytes int64) ([]byte, error) {
	defer req.Body.Close()

	body, err := io.ReadAll(io.LimitReader(req.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, ErrRequestBodyTooLarge
	}

	return body, nil
}

func parseRequest(req *http.Request, path string, maxBodyBytes int64) (ParsedRequest, error) {
	headers := getRedactedHeaders(req)
	contentType := getContentType(headers)
	rawQuery := req.URL.RawQuery
	rawBody, err := readBody(req, maxBodyBytes)
	if err != nil {
		return ParsedRequest{}, fmt.Errorf("read request body: %w", err)
	}

	return ParsedRequest{
		Method:      reqMethod(req.Method),
		Path:        path,
		ReceiptTime: time.Now().UTC().Truncate(time.Second),
		RawQuery:    rawQuery,
		Headers:     headers,
		ContentType: contentType,
		RawBody:     rawBody,
		BodySizeKiB: (len(rawBody) + 1023) / 1024,
	}, nil
}
