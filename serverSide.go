package httpStreamWriter

import (
	"net/http"
)

func HijackAndForceClose(w http.ResponseWriter) {

	hj, ok := w.(http.Hijacker)
	if !ok {
		logger.Println("Error hijacking connection")
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		logger.Println("Error hijacking connection. Error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	conn.Close()
}
