package egw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/egw-ws/internal/allocator"
	"acnodal.io/egw-ws/internal/egw/db"
	"acnodal.io/egw-ws/internal/model"
	"acnodal.io/egw-ws/internal/util"
)

var (
	duplicateRE = regexp.MustCompile(`^.*duplicate endpoint: (.*)$`)
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
	Endpoint egwv1.Endpoint
}

// createService handles PureLB service announcements. They're sent
// from the EGW pool in the allocator, so we need to allocate and
// return the LB address.
func (g *EGW) createService(w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		body ServiceCreateRequest
	)
	vars := mux.Vars(r)

	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		fmt.Printf("POST service failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	// get the owning group which points to the service prefix from
	// which we'll allocate the address
	group, err := db.ReadGroup(r.Context(), g.client, vars["account"], vars["group"])
	if err != nil {
		fmt.Printf("POST service failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	// allocate a public IP address for the service
	addr, err := g.allocator.AllocateFromPool(body.Service.Name, group.Group.Spec.ServicePrefix, body.Service.Spec.PublicPorts, "")
	if err != nil {
		fmt.Printf("POST service failed %#v\n", err)
		util.RespondError(w, err)
		return
	}
	body.Service.Spec.PublicAddress = addr.String()

	// make sure that the link to the owning service group is set
	body.Service.Spec.ServiceGroup = vars["group"]

	// create the service CR
	err = db.CreateService(r.Context(), g.client, vars["account"], body.Service)
	if err != nil {
		fmt.Printf("POST service failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST service created %v %#v\n", vars["account"], body.Service)
	http.Redirect(w, r, fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], body.Service.ObjectMeta.Name), http.StatusFound)
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
	fmt.Printf("GET service failed %s/%s %#v\n", vars["account"], vars["service"], err)
	util.RespondError(w, err)
}

func (g *EGW) deleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := db.DeleteService(r.Context(), g.client, vars["account"], vars["service"])
	if err == nil {
		fmt.Printf("DELETE service %s/%s\n", vars["account"], vars["service"])
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

	// Tie the endpoint to the service. We use a label so we can query
	// for the set of endpoints that belong to a given LB.
	body.Endpoint.Labels = map[string]string{egwv1.OwningLoadBalancerLabel: service.Service.Name}
	body.Endpoint.Spec.LoadBalancer = service.Service.Name

	// Give the endpoint a name
	if body.Endpoint.Name == "" {
		name, err := uuid.NewRandom()
		if err != nil {
			fmt.Printf("generating uuid: %s\n", err)
			util.RespondError(w, err)
			return
		}
		body.Endpoint.Name = fmt.Sprintf("%s-%s", body.Endpoint.Spec.LoadBalancer, name.String())
	}

	// Create the endpoint
	err = db.CreateEndpoint(ctx, g.client, vars["account"], body.Endpoint)
	if err != nil {
		matches := duplicateRE.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("Duplicate endpoint %#v: %s\n", body.Endpoint, err)

			// We already had that endpoint, but we can return what we hope
			// the client needs to set up the tunnels on its end
			links := model.Links{"self": fmt.Sprintf("%s/%s", r.RequestURI, matches[1])}
			util.RespondConflict(w, map[string]interface{}{"message": err.Error(), "link": links, "endpoint": body.Endpoint}, util.EmptyHeader)
			return
		}

		// Something else went wrong
		fmt.Printf("POST endpoint failed %#v %#v\n", body, err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST endpoint created %#v\n", body.Endpoint)
	http.Redirect(w, r, fmt.Sprintf("/api/egw/accounts/%v/services/%v/endpoints/%v", vars["account"], vars["service"], body.Endpoint.Name), http.StatusFound)
	return
}

func (g *EGW) showEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ep, err := db.ReadEndpoint(r.Context(), g.client, vars["account"], vars["endpoint"])
	if err == nil {
		ep.Links = model.Links{"self": r.RequestURI, "service": fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], vars["service"])}
		util.RespondJSON(w, http.StatusOK, ep, util.EmptyHeader)
		return
	}
	fmt.Printf("GET endpoint failed %s/%s %#v\n", vars["account"], vars["endpoint"], err)
	util.RespondError(w, err)
}

func (g *EGW) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := db.DeleteEndpoint(r.Context(), g.client, vars["account"], vars["endpoint"])
	if err == nil {
		fmt.Printf("DELETE endpoint %s/%s\n", vars["account"], vars["endpoint"])
		util.RespondJSON(w, http.StatusOK, map[string]string{"message": "endpoint deleted"}, util.EmptyHeader)
		return
	}
	util.RespondError(w, err)
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
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/endpoints/{endpoint}", egw.deleteEndpoint).Methods(http.MethodDelete)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/endpoints/{endpoint}", egw.showEndpoint).Methods(http.MethodGet)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/endpoints", egw.createServiceEndpoint).Methods(http.MethodPost)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}", egw.deleteService).Methods(http.MethodDelete)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}", egw.showService).Methods(http.MethodGet)
	egwRouter.HandleFunc("/accounts/{account}/groups/{group}/services", egw.createService).Methods(http.MethodPost)
	egwRouter.HandleFunc("/accounts/{account}/groups/{group}", egw.showGroup).Methods(http.MethodGet)
}
