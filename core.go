package httpStreamWriter

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

//Composite write closer is a combination of a downstream writer and relevant closers meant to propegate the io.EOF signal downstream
type compositeWriteCloser struct {
	Writer  io.Writer
	Closers []io.Closer
}

func (cwc compositeWriteCloser) Write(b []byte) (int, error) {
	return cwc.Writer.Write(b)
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
		return errors.New(compoundErr)
	} else {
		return nil
	}
}

func HttpStreamWriter(target *url.URL, boundary string, extraHeaders map[string]string, responseFunc func(r *http.Response, err error)) (io.WriteCloser, error) {
	var err error

	client := &http.Client{
		Timeout: 0,
	}

	pipeRdr, pipeWrt := io.Pipe()

	req := &http.Request{
		Method: "POST",
		URL:    target,
		Body:   pipeRdr,
	}

	go func() {
		responseFunc(client.Do(req))
	}()

	mpWrt := multipart.NewWriter(pipeWrt)
	if err != nil {
		return nil, err
	}

	mpWrt.SetBoundary(boundary)
	partWrt, err := CreateRichFormFile(mpWrt, "Stream", extraHeaders)
	if err != nil {
		return nil, err
	}

	retWriteCloser := compositeWriteCloser{Writer: partWrt, Closers: []io.Closer{mpWrt, pipeWrt}}

	return retWriteCloser, nil
}

func CreateRichFormFile(w *multipart.Writer, fieldname string, extraHeaders map[string]string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, escapeQuotes(fieldname)))
	h.Set("Content-Type", "application/octet-stream")
	for k, v := range extraHeaders {
		h.Set(k, v)
	}
	return w.CreatePart(h)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}
