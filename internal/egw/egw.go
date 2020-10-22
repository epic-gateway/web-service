package egw

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/egw-ws/internal/allocator"
	"acnodal.io/egw-ws/internal/egw/db"
	"acnodal.io/egw-ws/internal/model"
	"acnodal.io/egw-ws/internal/util"
)

// EGW implements the server side of the EGW web service protocol.
type EGW struct {
	client    client.Client
	allocator *allocator.Allocator
}

// ServiceCreateRequest contains the data from a web service request
// to create a Service.
type ServiceCreateRequest struct {
	Service egwv1.LoadBalancer
}

// EndpointCreateRequest contains the data from a web service request
// to create a Endpoint.
type EndpointCreateRequest struct {
	Endpoint egwv1.LoadBalancerEndpoint
}

// createService handles PureLB service announcements. They're sent
// from the EGW pool in the allocator, so we need to allocate and
// return the LB address.
func (g *EGW) createService(w http.ResponseWriter, r *http.Request) {
	var (
		err error
	)
	vars := mux.Vars(r)

	var body ServiceCreateRequest
	err = json.NewDecoder(r.Body).Decode(&body)
	if err == nil {
		var addr net.IP

		// allocate a public IP address for the service
		_, addr, err = g.allocator.Allocate(body.Service.Name, body.Service.Spec.PublicPorts, "")
		if err == nil {
			body.Service.Spec.PublicAddress = addr.String()

			// make sure that the link to the owning service group is set
			body.Service.Spec.ServiceGroup = vars["group"]

			err = db.CreateService(context.Background(), g.client, vars["account"], body.Service)
			if err == nil {
				fmt.Printf("POST service created %v %#v\n", vars["account"], body.Service)
				http.Redirect(w, r, fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], body.Service.ObjectMeta.Name), http.StatusFound)
				return
			}
		}
	}
	fmt.Printf("POST service failed %#v\n", err)
	util.RespondError(w)
}

func (g *EGW) showService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	service, err := db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err == nil {
		service.Links = model.Links{
			"self":            fmt.Sprintf("%s", r.RequestURI),
			"group":           fmt.Sprintf("/api/egw/accounts/%v/groups/%v", vars["account"], service.Service.Spec.ServiceGroup), // FIXME: use gorilla mux "registered url" to build these urls
			"create-endpoint": fmt.Sprintf("%s/endpoints", r.RequestURI),
		}
		fmt.Printf("GET service %#v\n", service)
		util.RespondJSON(w, service)
		return
	}
	fmt.Printf("GET service failed %#v\n", err)
	util.RespondError(w)
}

func (g *EGW) deleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := db.DeleteService(r.Context(), g.client, vars["account"], vars["service"])
	if err == nil {
		util.RespondJSON(w, map[string]string{"message": "delete successful"})
		return
	}
	util.RespondError(w)
}

func (g *EGW) createServiceEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var body EndpointCreateRequest
	err := json.NewDecoder(r.Body).Decode(&body)
	if err == nil {
		fmt.Printf("POST creating endpoint %#v\n", body)
		err = db.CreateEndpoint(context.Background(), g.client, vars["account"], vars["service"], body.Endpoint)
		if err == nil {
			fmt.Printf("POST endpoint created %#v\n", body)
			http.Redirect(w, r, fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], vars["service"]), http.StatusFound)

			return
		}
	}
	fmt.Printf("POST endpoint failed %#v\n", err)
	util.RespondError(w)
}

func (g *EGW) showGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	group, err := db.ReadGroup(r.Context(), g.client, vars["account"], vars["group"])
	if err == nil {
		group.Links = model.Links{"self": r.RequestURI, "create-service": fmt.Sprintf("%s/%v", r.RequestURI, "services")}
		util.RespondJSON(w, group)
		return
	}
	util.RespondError(w)
}

// NewEGW configures a new EGW web service instance.
func NewEGW(client client.Client, allocator *allocator.Allocator) *EGW {
	return &EGW{client: client, allocator: allocator}
}

// SetupRoutes sets up the provided mux.Router to handle the EGW web
// service routes.
func SetupRoutes(router *mux.Router, prefix string, client client.Client, allocator *allocator.Allocator) {
	egw := NewEGW(client, allocator)
	egwRouter := router.PathPrefix(prefix).Subrouter()
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/endpoints", egw.createServiceEndpoint).Methods(http.MethodPost)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}", egw.deleteService).Methods(http.MethodDelete)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}", egw.showService).Methods(http.MethodGet)
	egwRouter.HandleFunc("/accounts/{account}/groups/{group}/services", egw.createService).Methods(http.MethodPost)
	egwRouter.HandleFunc("/accounts/{account}/groups/{group}", egw.showGroup).Methods(http.MethodGet)
}
