package egw

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/egw-ws/internal/egw/db"
	"acnodal.io/egw-ws/internal/model"
	"acnodal.io/egw-ws/internal/util"
)

var (
	multiClusterLB = regexp.MustCompile(`has upstream clusters, can't delete`)
	duplicateLB    = regexp.MustCompile(`^loadbalancers.egw.acnodal.io "(.*)" already exists$`)
	duplicateRep   = regexp.MustCompile(`^.*duplicate endpoint: (.*)$`)
	rfc1123Cleaner = strings.NewReplacer(".", "-", ":", "-")
)

// EGW implements the server side of the EGW web service protocol.
type EGW struct {
	client client.Client
}

// ServiceCreateRequest contains the data from a web service request
// to create a Service.
type ServiceCreateRequest struct {
	Service egwv1.LoadBalancer
}

// ClusterCreateRequest contains the data from a web service request
// to create an upstream cluster.
type ClusterCreateRequest struct {
	ClusterID string `json:"cluster-id"`
}

// EndpointCreateRequest contains the data from a web service request
// to create a Endpoint.
type EndpointCreateRequest struct {
	Endpoint egwv1.RemoteEndpoint
}

func toLower(protocol v1.Protocol) string {
	return strings.ToLower(string(protocol))
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
		util.RespondBad(w, err)
		return
	}

	// get the owning group which points to the service prefix from
	// which we'll allocate the address
	group, err := db.ReadGroup(r.Context(), g.client, vars["account"], vars["group"])
	if err != nil {
		fmt.Printf("POST service failed %#v\n", err)
		util.RespondNotFound(w, err)
		return
	}

	// If this LB can not be shared then give it a random readable name
	// so it doesn't collide with other LBs in the group that might be
	// created by other services with the same name. If the LB can be
	// shared then leave its name alone so other PureLB services with
	// the same name can find it. When they try to create their services
	// they'll get a 409 Conflict response that points them at this
	// service.
	if !group.Group.Spec.CanBeShared {
		raw := make([]byte, 8, 8)
		_, _ = rand.Read(raw)
		body.Service.Name += "-" + hex.EncodeToString(raw)
	}

	// Set links to the owning service group and prefix
	if body.Service.Labels == nil {
		body.Service.Labels = map[string]string{}
	}
	body.Service.Labels[egwv1.OwningServiceGroupLabel] = vars["group"]
	body.Service.Labels[egwv1.OwningServicePrefixLabel] = group.Group.Labels[egwv1.OwningServicePrefixLabel]

	// This LB will live in the same NS as its owning group
	body.Service.Namespace = group.Group.Namespace

	selfURL := fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], body.Service.ObjectMeta.Name)

	// Create the LB CR
	err = g.client.Create(r.Context(), &body.Service)
	if err != nil {
		matches := duplicateLB.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("Duplicate service %#v: %s\n", body.Service, err)

			// We already had that endpoint, but we can return what we hope
			// the client needs to set up the tunnels on its end
			util.RespondConflict(
				w,
				map[string]interface{}{"message": err.Error(), "link": model.Links{"self": selfURL}},
				map[string]string{"Location": selfURL},
			)
			return
		}

		// Something else went wrong
		fmt.Printf("POST service failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST service created %v %#v\n", vars["account"], body.Service.Spec)
	http.Redirect(w, r, selfURL, http.StatusFound)
}

func (g *EGW) showService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	service, err := db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err == nil {
		service.Links = model.Links{
			"self":            fmt.Sprintf("%s", r.RequestURI),
			"group":           fmt.Sprintf("/api/egw/accounts/%v/groups/%v", vars["account"], service.Service.Labels[egwv1.OwningServiceGroupLabel]), // FIXME: use gorilla mux "registered url" to build these urls
			"create-endpoint": fmt.Sprintf("%s/endpoints", r.RequestURI),
			"create-cluster":  fmt.Sprintf("%s/clusters", r.RequestURI),
		}
		fmt.Printf("GET service OK %s/%s\n", vars["account"], vars["service"])
		util.RespondJSON(w, http.StatusOK, service, util.EmptyHeader)
		return
	}
	fmt.Printf("GET service failed %s/%s %#v\n", vars["account"], vars["service"], err)
	util.RespondNotFound(w, err)
}

func (g *EGW) deleteService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Delete the CR
	if err := db.DeleteService(r.Context(), g.client, vars["account"], vars["service"]); err != nil {
		matches := multiClusterLB.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("service %s has clusters: %s\n", vars["service"], err)
			util.RespondConflict(w, map[string]interface{}{"message": err.Error()}, util.EmptyHeader)
			return
		}

		fmt.Printf("DELETE service failed %s/%s %#v\n", vars["account"], vars["service"], err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("DELETE service OK %s/%s\n", vars["account"], vars["service"])
	util.RespondJSON(w, http.StatusOK, map[string]string{"message": "delete successful"}, map[string]string{})
	return
}

func (g *EGW) createServiceCluster(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		service *model.Service
		body    ClusterCreateRequest
	)
	vars := mux.Vars(r)

	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		fmt.Printf("POST cluster failed %#v\n", err)
		util.RespondBad(w, err)
		return
	}

	// Validate the client cluster ID
	if body.ClusterID == "" {
		err := fmt.Errorf("cluster name not provided")
		fmt.Printf("POST cluster failed %#v\n", err)
		util.RespondBad(w, err)
		return
	}

	service, err = db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err != nil {
		fmt.Printf("POST cluster failed %s/%s/%s %#v\n", vars["account"], vars["service"], body.ClusterID, err)
		util.RespondNotFound(w, err)
	}

	selfURL := fmt.Sprintf("/api/egw/accounts/%s/services/%s/clusters/%s", vars["account"], vars["service"], body.ClusterID)

	if err := service.Service.AddUpstream(body.ClusterID); err != nil {
		fmt.Printf("Duplicate cluster %#v: %s\n", body.ClusterID, err)

		// The LB already had that cluster
		util.RespondConflict(
			w,
			map[string]interface{}{"message": err.Error(), "link": model.Links{"self": selfURL}},
			map[string]string{"Location": selfURL},
		)
		return
	}

	err = g.client.Update(r.Context(), &service.Service)
	if err != nil {
		// Something went wrong
		fmt.Printf("POST cluster failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST cluster OK %s/%s/%s\n", vars["account"], vars["service"], body.ClusterID)
	http.Redirect(w, r, selfURL, http.StatusFound)
}

func (g *EGW) showCluster(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		service *model.Service
	)
	vars := mux.Vars(r)

	// 404 if we can't find the service
	service, err = db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err != nil {
		fmt.Printf("GET cluster failed %s/%s/%s %#v\n", vars["account"], vars["service"], vars["cluster"], err)
		util.RespondNotFound(w, err)
	}

	// 404 if the service doesn't have a cluster with that name
	if !service.Service.ContainsUpstream(vars["cluster"]) {
		err = fmt.Errorf("cluster %s/%s/%s not found", vars["account"], vars["service"], vars["cluster"])
		fmt.Printf("GET cluster failed %#v\n", err)
		util.RespondNotFound(w, err)
		return
	}

	links := model.Links{"self": r.RequestURI, "service": fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], vars["service"])}
	fmt.Printf("GET cluster OK %s/%s/%s\n", vars["account"], vars["service"], vars["cluster"])
	util.RespondJSON(w, http.StatusOK, model.Cluster{Links: links}, util.EmptyHeader)
	return
}

func (g *EGW) deleteCluster(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		service *model.Service
	)
	vars := mux.Vars(r)

	service, err = db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err != nil {
		fmt.Printf("GET cluster failed %s/%s/%s %#v\n", vars["account"], vars["service"], vars["cluster"], err)
		util.RespondNotFound(w, err)
	}

	if err := service.Service.RemoveUpstream(vars["cluster"]); err != nil {
		fmt.Printf("GET cluster failed %#v\n", err)
		util.RespondNotFound(w, err)
		return
	}

	err = g.client.Update(r.Context(), &service.Service)
	if err != nil {
		// Something went wrong
		fmt.Printf("POST cluster failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("DELETE cluster OK %s/%s/%s\n", vars["account"], vars["service"], vars["cluster"])
	util.RespondJSON(w, http.StatusOK, map[string]string{"message": "delete successful"}, map[string]string{})
	return
}

func (g *EGW) createServiceEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var (
		body    EndpointCreateRequest
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
	service, err = db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err != nil {
		util.RespondNotFound(w, err)
		return
	}

	// Tie the endpoint to the service. We use a label so we can query
	// for the set of endpoints that belong to a given LB.
	body.Endpoint.Labels = map[string]string{egwv1.OwningLoadBalancerLabel: service.Service.Name}

	// Give the endpoint a name that's readable but also won't collide
	// with others
	if body.Endpoint.Name == "" {
		raw := make([]byte, 8, 8)
		_, _ = rand.Read(raw)
		body.Endpoint.Name = fmt.Sprintf("%s-%d-%s-%s", rfc1123Cleaner.Replace(body.Endpoint.Spec.Address), body.Endpoint.Spec.Port.Port, toLower(body.Endpoint.Spec.Port.Protocol), hex.EncodeToString(raw))
	}

	// This endpoint will live in the same NS as its owning LB
	body.Endpoint.Namespace = service.Service.Namespace

	// Create the endpoint
	err = g.client.Create(r.Context(), &body.Endpoint)
	if err != nil {
		matches := duplicateRep.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("Duplicate endpoint %#v: %s\n", body.Endpoint.Spec, err)

			// We already had that endpoint, but we can return what we hope
			// the client needs to set up the tunnels on its end
			otherURL := fmt.Sprintf("%s/%s", r.RequestURI, matches[1])
			util.RespondConflict(
				w,
				map[string]interface{}{"message": err.Error(), "link": model.Links{"self": otherURL}, "endpoint": body.Endpoint},
				map[string]string{"Location": otherURL},
			)
			return
		}

		// Something else went wrong
		fmt.Printf("POST endpoint failed %#v %#v\n", body, err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST endpoint created %#v\n", body.Endpoint.Spec)
	http.Redirect(w, r, fmt.Sprintf("/api/egw/accounts/%v/services/%v/endpoints/%v", vars["account"], vars["service"], body.Endpoint.Name), http.StatusFound)
	return
}

func (g *EGW) showEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ep, err := db.ReadEndpoint(r.Context(), g.client, vars["account"], vars["endpoint"])
	if err == nil {
		ep.Links = model.Links{"self": r.RequestURI, "service": fmt.Sprintf("/api/egw/accounts/%v/services/%v", vars["account"], vars["service"])}
		fmt.Printf("GET endpoint OK %s/%s\n", vars["account"], vars["endpoint"])
		util.RespondJSON(w, http.StatusOK, ep, util.EmptyHeader)
		return
	}
	fmt.Printf("GET endpoint failed %s/%s %#v\n", vars["account"], vars["endpoint"], err)
	util.RespondNotFound(w, err)
}

func (g *EGW) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := db.DeleteEndpoint(r.Context(), g.client, vars["account"], vars["endpoint"])
	if err == nil {
		fmt.Printf("DELETE endpoint %s/%s\n", vars["account"], vars["endpoint"])
		util.RespondJSON(w, http.StatusOK, map[string]string{"message": "endpoint deleted"}, util.EmptyHeader)
		return
	}
	fmt.Printf("DELETE endpoint failed %s/%s %#v\n", vars["account"], vars["endpoint"], err)
	util.RespondError(w, err)
}

func (g *EGW) showGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	group, err := db.ReadGroup(r.Context(), g.client, vars["account"], vars["group"])
	if err == nil {
		group.Links = model.Links{"self": r.RequestURI, "account": fmt.Sprintf("/api/egw/accounts/%v", vars["account"]), "create-service": fmt.Sprintf("%s/%v", r.RequestURI, "services")}
		util.RespondJSON(w, http.StatusOK, group, util.EmptyHeader)
		return
	}
	util.RespondNotFound(w, err)
}

func (g *EGW) showAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	account, err := db.ReadAccount(r.Context(), g.client, vars["account"])
	if err == nil {
		account.Links = model.Links{"self": r.RequestURI}
		util.RespondJSON(w, http.StatusOK, account, util.EmptyHeader)
		return
	}
	util.RespondNotFound(w, err)
}

// NewEGW configures a new EGW web service instance.
func NewEGW(client client.Client) *EGW {
	return &EGW{client: client}
}

// SetupRoutes sets up the provided mux.Router to handle the EGW web
// service routes.
func SetupRoutes(router *mux.Router, prefix string, client client.Client) {
	egw := NewEGW(client)
	egwRouter := router.PathPrefix(prefix).Subrouter()
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/endpoints/{endpoint}", egw.deleteEndpoint).Methods(http.MethodDelete)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/endpoints/{endpoint}", egw.showEndpoint).Methods(http.MethodGet)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/endpoints", egw.createServiceEndpoint).Methods(http.MethodPost)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/clusters/{cluster}", egw.deleteCluster).Methods(http.MethodDelete)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/clusters/{cluster}", egw.showCluster).Methods(http.MethodGet)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}/clusters", egw.createServiceCluster).Methods(http.MethodPost)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}", egw.deleteService).Methods(http.MethodDelete)
	egwRouter.HandleFunc("/accounts/{account}/services/{service}", egw.showService).Methods(http.MethodGet)
	egwRouter.HandleFunc("/accounts/{account}/groups/{group}/services", egw.createService).Methods(http.MethodPost)
	egwRouter.HandleFunc("/accounts/{account}/groups/{group}", egw.showGroup).Methods(http.MethodGet)
	egwRouter.HandleFunc("/accounts/{account}", egw.showAccount).Methods(http.MethodGet)
}
