package util

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func DumpBytes(label string, bytes io.Reader) error {
	var err error
	if b, err := ioutil.ReadAll(bytes); err == nil {
		log.Println(label + string(b))
	} else {
		log.Println(err)
	}
	return err
}

func RespondJson(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	bytes, err := json.Marshal(payload)
	if err != nil {
		RespondError(w)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

func RespondJsonString(w http.ResponseWriter, payload string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload))
}

func RespondError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
}
