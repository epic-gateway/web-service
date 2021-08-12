package controller

import (
	"net/http"

	"github.com/gorilla/mux"

	"acnodal.io/epic/web-service/internal/util"
)

func healthCheck(w http.ResponseWriter, r *http.Request) {
	util.RespondJSON(w, http.StatusOK, map[string]string{"healthy": "yes"}, util.EmptyHeader)
}

// SetupHealthzRoutes sets up the provided mux.Router to handle the
// health check route.
func SetupHealthzRoutes(router *mux.Router) {
	router.HandleFunc("/healthz", healthCheck).Methods(http.MethodGet).Name("healthz")
}
