package health

import (
	"database/sql"
	"net/http"
	"sync/atomic"
)

var ready atomic.Bool
var db *sql.DB

func Init(database *sql.DB) {
	db = database
}

func SetReady(readyFlag bool) {
	ready.Store(readyFlag)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if ready.Load() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}
}