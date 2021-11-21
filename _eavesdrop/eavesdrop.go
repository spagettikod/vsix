package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

func printHeaders(h http.Header) {
	for k, v := range h {
		fmt.Printf("%s: %v\n", k, strings.Join(v, ","))
	}
	fmt.Printf("\n")
}

func printBody(body []byte, isZip bool) error {
	if len(body) > 0 {
		if isZip {
			zr, err := gzip.NewReader(bytes.NewBuffer(body))
			if err != nil {
				return err
			}
			body, err = ioutil.ReadAll(zr)
			if err != nil {
				return err
			}
		}
		fmt.Println(string(body))
	}
	return nil
}

// logRequest logs the incoming request
func logRequest(r *http.Request) ([]byte, error) {
	fmt.Println("REQUEST ----------------------------------------")
	fmt.Printf("%s %s %s\n", r.Method, r.URL, r.Proto)
	printHeaders(r.Header)
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if strings.Index(r.Header.Get("Content-Type"), "application/json") == 0 {
		err = printBody(b, r.Header.Get("Content-Encoding") == "gzip")
	}
	return b, err
}

func logResponse(r *http.Response) ([]byte, error) {
	fmt.Println("RESPONSE ----------------------------------------")
	printHeaders(r.Header)
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if strings.Index(r.Header.Get("Content-Type"), "application/json") == 0 {
		err = printBody(b, r.Header.Get("Content-Encoding") == "gzip")
	}
	return b, err
}

// proxyRequest passes on the incoming request to Visual Studio Code Marketplace and returns the response.
func proxyRequest(method string, req *http.Request, header http.Header, body []byte) (*http.Response, error) {
	url := fmt.Sprintf("https://marketplace.visualstudio.com%s", req.URL.String())
	r, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	r.Header = header
	client := http.DefaultClient
	return client.Do(r)
}

// returnResponse passes on the response from Visual Studio Code Marketplace to the initial request.
func returnResponse(w http.ResponseWriter, headers http.Header, body []byte) error {
	for k, v := range headers {
		for _, hv := range v {
			w.Header().Add(k, hv)
		}
	}
	_, err := w.Write(body)
	return err
}

func handle(w http.ResponseWriter, r *http.Request) {
	body, err := logRequest(r)
	if err != nil {
		log.Println(err)
		http.Error(w, fmt.Sprintf("logRequest: %s", err), http.StatusInternalServerError)
		return
	}
	resp, err := proxyRequest(r.Method, r, r.Header, body)
	if err != nil {
		log.Println(err)
		http.Error(w, fmt.Sprintf("proxyRequest: %s", err), http.StatusInternalServerError)
		return
	}
	body, err = logResponse(resp)
	if err != nil {
		log.Println(err)
		http.Error(w, fmt.Sprintf("logResponse: %s", err), http.StatusInternalServerError)
		return
	}
	err = returnResponse(w, resp.Header, body)
	if err != nil {
		log.Println(err)
		http.Error(w, fmt.Sprintf("returnResponse: %s", err), http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/", handle)
	if err := http.ListenAndServeTLS(":8080", "/home/roland/lego/certificates/lisa.spagettikod.se.crt", "/home/roland/lego/certificates/lisa.spagettikod.se.key", nil); err != nil {
		log.Fatalln(err)
	}
}
