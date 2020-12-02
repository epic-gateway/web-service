package util

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// EmptyHeader is a convenient way to tell RespondJSON that you don't
// have any headers to send.
var EmptyHeader = map[string]string{}

// RespondJSON sends an HTTP response containing the payload,
// marshaled into JSON format.
func RespondJSON(w http.ResponseWriter, status int, payload interface{}, headers map[string]string) {
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	bytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshaling response %#v\n", err)
		bytes = []byte{}
	}
	w.WriteHeader(status)
	w.Write(bytes)
}

// RespondJSONString sends an HTTP response containing the payload,
// which should be JSON.
func RespondJSONString(w http.ResponseWriter, payload string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload))
}

// RespondProblem sends an HTTP response.
func RespondProblem(w http.ResponseWriter, code int, message string) {
	RespondJSON(w, code, map[string]string{"error": message}, EmptyHeader)
}

// RespondError sends an HTTP 5xx "internal server error" response.
func RespondError(w http.ResponseWriter, err error) {
	RespondProblem(w, http.StatusInternalServerError, err.Error())
}

// RespondBad sends an HTTP 4xx "bad request" response.
func RespondBad(w http.ResponseWriter, err error) {
	RespondProblem(w, http.StatusBadRequest, err.Error())
}

// RespondNotFound sends an HTTP 404 "not found" response.
func RespondNotFound(w http.ResponseWriter, err error) {
	RespondProblem(w, http.StatusNotFound, err.Error())
}

// RespondConflict sends an HTTP 409 "conflict" response.
func RespondConflict(w http.ResponseWriter, payload interface{}, headers map[string]string) {
	RespondJSON(w, http.StatusConflict, payload, headers)
}
