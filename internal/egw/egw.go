package egw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"

	"acnodal.io/egw-ws/internal/egw/db"
	"acnodal.io/egw-ws/internal/egw/model"
	"acnodal.io/egw-ws/internal/util"
)

type EGW struct {
	db *pgxpool.Pool
}

type ServiceCreateRequest struct {
	Service model.Service
}
type EndpointCreateRequest struct {
	Endpoint model.Endpoint
}

func (g *EGW) createService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	grpid, err := uuid.Parse(vars["id"])
	if err == nil {
		var body ServiceCreateRequest
		err = json.NewDecoder(r.Body).Decode(&body)
		if err == nil {
			body.Service.GroupID = grpid
			svcid, err := db.CreateService(context.Background(), g.db, body.Service)
			if err == nil {
				fmt.Printf("POST service created %v %#v\n", svcid, body.Service)
				http.Redirect(w, r, fmt.Sprintf("/api/egw/services/%v", svcid), http.StatusFound)
				return
			}
		}
	}
	fmt.Printf("POST service failed %#v\n", err)
	util.RespondError(w)
}

func (g *EGW) showService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err == nil {
		service, err := db.ReadService(context.Background(), g.db, id)
		if err == nil {
			service.Links = model.Links{
				"self":  fmt.Sprintf("%s", r.RequestURI),
				"group": fmt.Sprintf("/api/egw/groups/%v", service.GroupID), // FIXME: use gorilla mux "registered url" to build the url
				"create-endpoint": fmt.Sprintf("%s/endpoints", r.RequestURI), // FIXME: use gorilla mux "registered url" to build the url
			}
			fmt.Printf("GET service %#v\n", service)
			util.RespondJson(w, service)
			return
		}
	}
	fmt.Printf("GET service failed %#v\n", err)
	util.RespondError(w)
}

func (g *EGW) createEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	svcid, err := uuid.Parse(vars["id"])
	if err == nil {
		var body EndpointCreateRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		if err == nil {
			fmt.Printf("POST creating endpoint %#v\n", body)
			body.Endpoint.ServiceID = svcid
			epid, err := db.CreateEndpoint(context.Background(), g.db, body.Endpoint)
			if err == nil {
				fmt.Printf("POST endpoint created %#v\n", body)
				http.Redirect(w, r, fmt.Sprintf("/api/egw/endpoints/%v", epid), http.StatusFound)

				return
			}
		}
	}
	fmt.Printf("POST endpoint failed %#v\n", err)
	util.RespondError(w)
}

func (g *EGW) showEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err == nil {
		endpoint, err := db.ReadEndpoint(context.Background(), g.db, id)
		if err == nil {
			endpoint.Links = model.Links{
				"self":  fmt.Sprintf("%s", r.RequestURI),
				"service": fmt.Sprintf("/api/egw/services/%v", endpoint.ServiceID), // FIXME: use gorilla mux "registered url" to build the url
			}
			fmt.Printf("GET endpoint %#v\n", endpoint)
			util.RespondJson(w, endpoint)
			return
		}
	}
	fmt.Printf("GET endpoint failed %#v\n", err)
	util.RespondError(w)
}

func (g *EGW) showGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err == nil {
		group, err := db.ReadGroup(context.Background(), g.db, id)
		if err == nil {
			group.Links = model.Links{"self": r.RequestURI, "create-service": fmt.Sprintf("%s/%v", r.RequestURI, "services")}
			util.RespondJson(w, group)
			return
		}
	}
	util.RespondError(w)
}

func NewEGW(pool *pgxpool.Pool) *EGW {
	return &EGW{db: pool}
}

func SetupRoutes(router *mux.Router, prefix string, pool *pgxpool.Pool) {
	egw := NewEGW(pool)
	egw_router := router.PathPrefix(prefix).Subrouter()
	egw_router.HandleFunc("/endpoints/{id}", egw.showEndpoint).Methods(http.MethodGet)
	egw_router.HandleFunc("/services/{id}/endpoints", egw.createEndpoint).Methods(http.MethodPost)
	egw_router.HandleFunc("/services/{id}", egw.showService).Methods(http.MethodGet)
	egw_router.HandleFunc("/groups/{id}/services", egw.createService).Methods(http.MethodPost)
	egw_router.HandleFunc("/groups/{id}", egw.showGroup).Methods(http.MethodGet)
}
