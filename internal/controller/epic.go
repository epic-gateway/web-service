package controller

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/epic/web-service/internal/db"
	"acnodal.io/epic/web-service/internal/model"
	"acnodal.io/epic/web-service/internal/util"
)

var (
	multiClusterLB = regexp.MustCompile(`has upstream clusters, can't delete`)
)

// EPIC implements the server side of the EPIC web service protocol.
type EPIC struct {
	client client.Client
	router *mux.Router
}

func (g *EPIC) showGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	group, err := db.ReadGroup(r.Context(), g.client, vars["account"], vars["group"])
	if err == nil {
		acctLink, err := g.router.Get("account").URL("account", vars["account"])
		if err != nil {
			fmt.Printf("GET group failed %s/%s: %s\n", vars["account"], vars["group"], err)
			util.RespondError(w, err)
			return
		}
		proxyLink, err := g.router.Get("group-proxies").URL("account", vars["account"], "group", vars["group"])
		if err != nil {
			fmt.Printf("GET group failed %s/%s: %s\n", vars["account"], vars["group"], err)
			util.RespondError(w, err)
			return
		}
		group.Links = model.Links{
			"self":         r.RequestURI,
			"account":      acctLink.String(),
			"create-proxy": proxyLink.String(),
		}
		util.RespondJSON(w, http.StatusOK, group, util.EmptyHeader)
		return
	}
	util.RespondNotFound(w, err)
}

func (g *EPIC) showAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	account, err := db.ReadAccount(r.Context(), g.client, vars["account"])
	if err == nil {
		routeLink, err := g.router.Get("account-routes").URL("account", vars["account"])
		if err != nil {
			fmt.Printf("GET account failed %s: %s\n", vars["account"], err)
			util.RespondError(w, err)
			return
		}
		sliceLink, err := g.router.Get("account-slices").URL("account", vars["account"])
		if err != nil {
			fmt.Printf("GET account failed %s: %s\n", vars["account"], err)
			util.RespondError(w, err)
			return
		}
		account.Links = model.Links{
			"self":         r.RequestURI,
			"create-route": routeLink.String(),
			"create-slice": sliceLink.String(),
		}
		util.RespondJSON(w, http.StatusOK, account, util.EmptyHeader)
		return
	}
	util.RespondNotFound(w, err)
}

// NewEPIC configures a new EPIC web service instance.
func NewEPIC(client client.Client, router *mux.Router) *EPIC {
	return &EPIC{client: client, router: router}
}

// SetupEPICRoutes sets up the provided mux.Router to handle the web
// service routes.
func SetupEPICRoutes(router *mux.Router, client client.Client) {
	epic := NewEPIC(client, router)

	router.HandleFunc("/accounts/{account}/groups/{group}", epic.showGroup).Methods(http.MethodGet).Name("group")

	router.HandleFunc("/accounts/{account}", epic.showAccount).Methods(http.MethodGet).Name("account")
}
