# httpStreamWriter

Treat a HTTP endpoint like any other golang writer, sending bytestream through standard HTTP POST request. 


**Example**
```
	u, _ := url.Parse("http://your.url.address/streams")
	boundary = "JwTrSdsmXBsijUljzOQsjtJmqpMvbGHqgxXm"
	extraHeadings = make(map[string]string)
	extraHeadings["Extra-Heading"] = "Extra Heading Data"

	wrt, err := HttpStreamWriter(u, boundary, extraHeadings, func(res *http.Response, error) {
			res, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println("Trouble reading response.  Error: ", err)
			}
			fmt.Printf("response:\n\t%s\n", res)
		})

	// wrt is a writecloser you can now write to
```

HttpStreamWriterOK allows server to respond or refuse connection.