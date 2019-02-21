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

const boundary = "JwnftdsGXBsijUljzOQsjtJmqZMvbGHqgxXn"

var current = 0
var tests = []struct {
	inFile                 string
	outFile                string
	bytesExpected          int
	evenliftMetadataHeader string
}{
	{"toSend.txt", "received.txt", 39, "s:133;u:12"},
	{"bigVid.h264", "receivedVid.h264", 23052288, "{station:133,user:133,host:{name:\"henry\",more:\"here\"}}"},
}

func TestReceive(t *testing.T) {
	http.HandleFunc("/", streamHandler)

	go func() {
		if err := http.ListenAndServe(":9191", nil); err != nil {
			panic(err)
		}
	}()

	for i, test := range tests {
		current = i
		send(test.evenliftMetadataHeader, test.inFile, t)
	}
}

func responseFunc(r *http.Response, err error) {
	res, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("trouble reading response.  Error: ", err)
	}
	fmt.Printf("response:\n\t%s\n", res)
}

func send(metadata string, fileToOpen string, t *testing.T) {
	u, err := url.Parse("http://localhost:9191/")
	if err != nil {
		t.Error("Can't parse URL")
		return
	}
	eh := make(map[string]string)
	eh["Evenlift-Metadata"] = metadata

	wrt, err := HttpStreamWriter(u, boundary, eh, responseFunc)
	if err != nil {
		t.Error("Error creating writer")
		return
	}

	// Simulated stream
	f, err := os.Open(fileToOpen)
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
	f, err := os.Create(tests[current].outFile)

	if err != nil {
		panic(err)
	}

	partReader := multipart.NewReader(r.Body, boundary)
	buf := make([]byte, 256)
	for {
		part, err := partReader.NextPart()
		if err == io.EOF {
			fmt.Println("Got to MIME EOF")
			break
		}

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
