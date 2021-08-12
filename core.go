package httpStreamWriter

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"time"
)

var logger *log.Logger

//Composite write closer is a combination of a downstream writer and relevant closers meant to propegate the io.EOF signal downstream
type compositeWriteCloser struct {
	Writer  io.Writer
	Closers []io.Closer
}

func init() {
	logger = log.New(os.Stdout, "", log.Llongfile|log.Ldate|log.Ltime)
}

func (cwc compositeWriteCloser) Write(b []byte) (int, error) {
	n, err := cwc.Writer.Write(b)
	return n, err
}

func (cwc compositeWriteCloser) Close() error {
	var err error
	var compoundErr string
	for i, _ := range cwc.Closers {
		err = cwc.Closers[i].Close()
		if err != nil {
			compoundErr += fmt.Sprintf("Error on writer %v, error: %v;  ", cwc.Closers[i], err)
		}
	}

	if compoundErr == "" {
		return nil
	} else {
		return errors.New(compoundErr)
	}
}

func HttpStreamWriter(target *url.URL, boundary string, extraHTTPHeaders map[string]string, extraMIMEHeaders map[string]string, responseFunc func(r *http.Response, err error)) (io.WriteCloser, error) {
	var err error

	tr := http.DefaultTransport

	client := &http.Client{
		Transport: tr,
		Timeout:   0,
	}

	pipeRdr, pipeWrt := io.Pipe()

	req := &http.Request{
		Method:        "POST",
		URL:           target,
		Body:          pipeRdr,
		ProtoMajor:    1,
		ProtoMinor:    1,
		ContentLength: -1,
	}

	req.Header = make(map[string][]string)

	for k, v := range extraHTTPHeaders {
		req.Header.Add(k, v)
	}

	go func() {
		responseFunc(client.Do(req))
	}()

	mpWrt := multipart.NewWriter(pipeWrt)
	if err != nil {
		return nil, err
	}

	mpWrt.SetBoundary(boundary)

	partWrt, err := CreateRichFormFile(mpWrt, "Stream", extraMIMEHeaders)
	if err != nil {
		return nil, err
	}

	retWriteCloser := compositeWriteCloser{Writer: partWrt, Closers: []io.Closer{mpWrt, pipeWrt}}

	return retWriteCloser, nil
}

func HttpStreamWriterOk(target *url.URL, boundary string, extraHTTPHeaders map[string]string, extraMIMEHeaders map[string]string, responseFunc func(r *http.Response, err error), duration time.Duration) (io.WriteCloser, error) {
	var err error

	tr := http.DefaultTransport

	client := &http.Client{
		Transport: tr,
		Timeout:   0,
	}

	pipeRdr, pipeWrt := io.Pipe()

	req := &http.Request{
		Method:        "POST",
		URL:           target,
		Body:          pipeRdr,
		ProtoMajor:    1,
		ProtoMinor:    1,
		ContentLength: -1,
	}

	req.Header = make(map[string][]string)

	for k, v := range extraHTTPHeaders {
		req.Header.Add(k, v)
	}

	errChan := make(chan error, 1)

	go func() {
		res, err := client.Do(req)

		if err != nil {
			errChan <- err
		} else if res.StatusCode == 404 || res.StatusCode == 500 {
			errChan <- errors.New(fmt.Sprintf("HttpStreamWriter Request failure. Status %v", res.StatusCode))
		}

		responseFunc(res, err)
	}()

	mpWrt := multipart.NewWriter(pipeWrt)
	if err != nil {
		return nil, err
	}

	mpWrt.SetBoundary(boundary)

	partWrt, err := CreateRichFormFile(mpWrt, "Stream", extraMIMEHeaders)
	if err != nil {
		return nil, err
	}

	retWriteCloser := compositeWriteCloser{Writer: partWrt, Closers: []io.Closer{mpWrt, pipeWrt}}

	select {
	case err := <-errChan:
		logger.Printf("Error creating http Stream Connection.  Error: %v", err)
		return retWriteCloser, err
	case <-time.After(duration):
		return retWriteCloser, nil
	}
}

func CreateRichFormFile(w *multipart.Writer, fieldname string, extraMIMEHeaders map[string]string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldname), escapeQuotes("Test")))
	h.Set("Content-Type", "application/octet-stream")
	for k, v := range extraMIMEHeaders {
		h.Set(k, v)
	}
	return w.CreatePart(h)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}
