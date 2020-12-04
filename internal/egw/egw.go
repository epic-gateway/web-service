package egw

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	"k8s.io/apimachinery/pkg/types"
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
	util.RespondError(w, err)
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
		util.RespondJSON(w, http.StatusOK, service, util.EmptyHeader)
		return
	}
	fmt.Printf("GET service failed %#v\n", err)
	util.RespondError(w, err)
}

func (g *EGW) deleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := db.DeleteService(r.Context(), g.client, vars["account"], vars["service"])
	if err == nil {
		util.RespondJSON(w, http.StatusOK, map[string]string{"message": "delete successful"}, map[string]string{})
		return
	}
	util.RespondError(w, err)
}

func (g *EGW) createServiceEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var (
		body    EndpointCreateRequest
		ctx     context.Context = context.Background()
		err     error
		service *model.Service
	)

	// Parse request
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		util.RespondBad(w, err)
		return
	}

	// Read the service to which this endpoint will belong
	service, err = db.ReadService(ctx, g.client, vars["account"], vars["service"])
	if err != nil {
		util.RespondNotFound(w, err)
		return
	}

	// Add the endpoint to the service
	err = service.Service.Spec.AddEndpoint(body.Endpoint)
	if err != nil {
		fmt.Printf("Duplicate endpoint %#v\n", body.Endpoint)

		// We already had that endpoint, but we can return what we hope
		// the client needs to set up the tunnels on its end
		util.RespondConflict(w, map[string]interface{}{"message": err.Error(), "service": service.Service}, util.EmptyHeader)
		return
	}

	// prepare a patch to add this endpoint to the LB spec
	patchBytes, err := json.Marshal([]map[string]interface{}{{"op": "add", "path": "/spec/endpoints/0", "value": body.Endpoint}})
	if err != nil {
		fmt.Printf("POST marshaling endpoint patch %#v\n", err)
		util.RespondError(w, err)
		return
	}

	// apply the patch
	fmt.Printf("POST creating endpoint %#v\n", body)
	err = g.client.Patch(ctx, &service.Service, client.RawPatch(types.JSONPatchType, patchBytes))
	if err != nil {
		fmt.Printf("POST endpoint failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST endpoint created %#v\n", body)
	http.Redirect(w, r, fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], vars["service"]), http.StatusFound)
	return
}

func (g *EGW) showGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	group, err := db.ReadGroup(r.Context(), g.client, vars["account"], vars["group"])
	if err == nil {
		group.Links = model.Links{"self": r.RequestURI, "create-service": fmt.Sprintf("%s/%v", r.RequestURI, "services")}
		util.RespondJSON(w, http.StatusOK, group, util.EmptyHeader)
		return
	}
	util.RespondError(w, err)
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
