package utils

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	API_URL string = "https://api.finhacks.id"
)

type Request struct {
	Url     string
	Path    string
	Method  string
	Headers map[string]string
	Param   url.Values
	Json    interface{}
	IsJson  bool
}

func NewHTTPRequest() *Request {
	return &Request{
		Headers: map[string]string{},
	}
}

func (r *Request) JSONPost(u *url.URL) (*http.Request, error) {
	u.Path += r.Path
	link := u.String()
	body, err := json.Marshal(r.Json)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(r.Method, link, bytes.NewReader(body))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	r.Headers["Content-Type"] = "application/json"

	return req, nil
}

func (r *Request) Post(u *url.URL) (*http.Request, error) {
	if r.IsJson {
		return r.JSONPost(u)
	}
	u.Path += r.Path
	link := u.String()
	form := strings.NewReader(r.Param.Encode())
	req, err := http.NewRequest(r.Method, link, form)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return req, nil
}

func (r *Request) Get(u *url.URL) (*http.Request, error) {
	u.RawQuery = r.Param.Encode()
	u.Path += r.Path
	link := u.String()
	req, err := http.NewRequest(r.Method, link, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return req, nil
}

// DoReq is Last Point to call api
func (r *Request) DoReq() (*[]byte, error) {
	u, err := url.Parse(r.Url)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	var req *http.Request
	switch r.Method {
	case "GET":
		req, err = r.Get(u)
	case "POST", "PUT":
		req, err = r.Post(u)
	}

	if req == nil {
		return nil, fmt.Errorf("Failed create new request")
	}

	if err != nil {
		return nil, err
	}

	for key, value := range r.Headers {
		req.Header.Set(key, value)
	}

	// For intermittent EOF error
	req.Close = true

	hc := &http.Client{
		Timeout: time.Duration(20 * time.Second),
	}

	resp, err := hc.Do(req)
	if err != nil {
		log.Println(resp, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Println(resp)
		printErrorBody(resp)
		return nil, fmt.Errorf("Status Code = %d", resp.StatusCode)
	}

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &contents, nil
}

func printErrorBody(resp *http.Response) {
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(string(content))
	return
}
