package util

import (
	"encoding/json"
	"net/http"
)

// RespondJSON sends an HTTP response containing the payload,
// marshaled into JSON format.
func RespondJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	bytes, err := json.Marshal(payload)
	if err != nil {
		RespondError(w)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

// RespondJSONString sends an HTTP response containing the payload,
// which should be JSON.
func RespondJSONString(w http.ResponseWriter, payload string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload))
}

// RespondError sends an HTTP 5xx "internal server error" response.
func RespondError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
}

// RespondBad sends an HTTP 4xx "bad request" response.
func RespondBad(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
}
