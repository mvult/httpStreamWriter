package httpStreamWriter

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

const boundary = "JwnftdsGXBsijUljzOQsjqJmqZMvbGHqgxXn"

var current = 0

type Test struct {
	inFile                 string
	outFile                string
	bytesExpected          int
	evenliftMetadataHeader string
	url                    string
	confirm                bool
	duration               time.Duration
	expectedError          bool
}

var tests = []Test{
	{"toSend.txt", "received.txt", 39, "s:133;u:12", "http://localhost:9191/base", false, time.Duration(time.Second), false},
	{"bigVid.h264", "receivedVid.h264", 23052288, "{station:133,user:133,host:{name:\"henry\",more:\"here\"}}", "http://localhost:9191/base", false, time.Duration(time.Second), false},
	{"bigVid.h264", "receivedVid.h264", 23052288, "reject", "http://localhost:9191/there", true, time.Duration(time.Second * 100), true},
}

func TestReceive(t *testing.T) {
	http.HandleFunc("/base", streamHandler)

	go func() {
		if err := http.ListenAndServe(":9191", nil); err != nil {
			panic(err)
		}
	}()

	for i, test := range tests {
		current = i
		send(test, t)
	}
}

func responseFunc(r *http.Response, err error) {
	logger.Println(r)
	logger.Println(err)
	if err != nil {
		return
	}
	res, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("trouble reading response.  Error: ", err)
	}
	fmt.Printf("response:\n\t%s\n", res)
}

func send(test Test, t *testing.T) {
	u, err := url.Parse(test.url)
	if err != nil {
		t.Error("Can't parse URL")
		return
	}
	eh := make(map[string]string)
	eh["Evenlift-Metadata"] = test.evenliftMetadataHeader
	mimeHeaders := map[string]string{}

	var wrt io.WriteCloser

	if test.confirm {
		wrt, err = HttpStreamWriterOk(u, boundary, eh, mimeHeaders, responseFunc, test.duration)
		if err == nil && test.expectedError {
			t.Error("Expected error but didn't get it.")
			return
		}

		if err != nil && !test.expectedError {
			t.Error("Unexpected error")
			return
		}

		if test.expectedError && err != nil {
			logger.Printf("Succesfully received expected error: %v\n", err)
			return
		}
	} else {
		wrt, err = HttpStreamWriter(u, boundary, eh, mimeHeaders, responseFunc)
		if err != nil {
			t.Error("Error creating writer")
			return
		}
	}

	// Simulated stream
	f, err := os.Open(test.inFile)
	if err != nil {
		t.Error("Can't open file")
		return
	}

	b := make([]byte, 1024)
	for {
		n, err := f.Read(b)
		if err != nil {
			if err == io.EOF {
				wrt.Close()
				break
			} else {
				panic(err)
			}
		}
		if _, err = wrt.Write(b[:n]); err != nil {
			panic(err)
		}
	}

	time.Sleep(time.Second * 3)
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	var n, total int

	fmt.Println("Receiving stream")

	if r.Header["Evenlift-Metadata"][0] == "reject" {
		fmt.Println("Returning")
		HijackAndForceClose(w)
		return
	}

	f, err := os.Create(tests[current].outFile)

	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", r)
	fmt.Printf("%+v\n", r.Header)
	partReader := multipart.NewReader(r.Body, boundary)
	fmt.Printf("%+v\n", partReader)
	buf := make([]byte, 256)
	for {
		part, err := partReader.NextPart()
		if err == io.EOF {
			fmt.Println("Got to MIME EOF")
			break
		}
		fmt.Printf("%+v\n", part)
		fmt.Println(part.Header)

		for {
			n, err = part.Read(buf)
			if err != nil {
				if err == io.EOF {
					fmt.Println("Got to part EOF")
					break
				}
				panic(err)
			}
			n, err := f.Write(buf[:n])
			if err != nil {
				panic(err)
			}
			total += n
		}
		n, err = f.Write(buf[:n])
		if err != nil {
			panic(err)
		}
		total += n
	}
	// fmt.Printf("Wrote %v bytes\n", total)
	if total != tests[current].bytesExpected {
		panic("Unexpected number of bytes written")
	}
	if _, err = w.Write([]byte(fmt.Sprintf("Wrote %v bytes", total))); err != nil {
		panic(err)
	}

	f.Close()
	if err = os.Remove(tests[current].outFile); err != nil {
		panic(err)
	}
}
