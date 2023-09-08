package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"

	epicv1 "epic-gateway.org/resource-model/api/v1"
	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/epic/web-service/internal/db"
	"acnodal.io/epic/web-service/internal/model"
	"acnodal.io/epic/web-service/internal/util"
)

var (
	multiClusterLB = regexp.MustCompile(`has upstream clusters, can't delete`)
	duplicateLB    = regexp.MustCompile(`^loadbalancers.epic.acnodal.io "(.*)" already exists$`)
	duplicateRep   = regexp.MustCompile(`^.*duplicate endpoint: (.*)$`)
)

// EPIC implements the server side of the EPIC web service protocol.
type EPIC struct {
	client client.Client
	router *mux.Router
}

// ServiceCreateRequest contains the data from a web service request
// to create a Service.
type ServiceCreateRequest struct {
	Service epicv1.LoadBalancer
}

// ClusterCreateRequest contains the data from a web service request
// to create an upstream cluster.
type ClusterCreateRequest struct {
	ClusterID string `json:"cluster-id"`
}

// EndpointCreateRequest contains the data from a web service request
// to create a Endpoint.
type EndpointCreateRequest struct {
	Endpoint epicv1.RemoteEndpoint
}

// createService handles PureLB service announcements. They're sent
// from the allocator pool, so we need to allocate and return the LB
// address.
func (g *EPIC) createService(w http.ResponseWriter, r *http.Request) {
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

	body.Service.Name = epicv1.LoadBalancerName(vars["group"], body.Service.Name, group.Group.Spec.CanBeShared)

	// Set links to the owning service group and prefix
	if body.Service.Labels == nil {
		body.Service.Labels = map[string]string{}
	}
	body.Service.Labels[epicv1.OwningLBServiceGroupLabel] = vars["group"]
	body.Service.Labels[epicv1.OwningServicePrefixLabel] = group.Group.Labels[epicv1.OwningServicePrefixLabel]

	// This LB will live in the same NS as its owning group
	body.Service.Namespace = group.Group.Namespace

	selfURL, err := g.router.Get("service").URL("account", vars["account"], "service", body.Service.ObjectMeta.Name)
	if err != nil {
		fmt.Printf("POST service failed %s/%s: %s\n", vars["account"], vars["group"], err)
		util.RespondError(w, err)
		return
	}

	// Create the LB CR
	err = g.client.Create(r.Context(), &body.Service)
	if err != nil {
		matches := duplicateLB.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("POST service 409/duplicate %s/%s\n", vars["account"], body.Service.Name)

			// We already had that endpoint, but we can return what we hope
			// the client needs to set up the tunnels on its end
			util.RespondConflict(
				w,
				map[string]interface{}{"message": err.Error(), "link": model.Links{"self": selfURL.String()}},
				map[string]string{"Location": selfURL.String()},
			)
			return
		}

		// Something else went wrong
		fmt.Printf("POST service failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST service OK %v %#v\n", vars["account"], body.Service.Spec)
	http.Redirect(w, r, selfURL.String(), http.StatusFound)
}

func (g *EPIC) showService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	service, err := db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err == nil {
		groupLink, err := g.router.Get("group").URL("account", vars["account"], "group", service.Service.Labels[epicv1.OwningLBServiceGroupLabel])
		if err != nil {
			fmt.Printf("GET service failed %s/%s: %s\n", vars["account"], vars["group"], err)
			util.RespondError(w, err)
			return
		}
		service.Links = model.Links{
			"self":            fmt.Sprintf("%s", r.RequestURI),
			"group":           groupLink.String(),
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

func (g *EPIC) deleteService(w http.ResponseWriter, r *http.Request) {
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

func (g *EPIC) createServiceCluster(w http.ResponseWriter, r *http.Request) {
	var (
		err        error
		service    *model.Service
		body       ClusterCreateRequest
		patchBytes []byte
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

	// Calculate our "self" URL
	selfURL, err := g.router.Get("cluster").URL("account", vars["account"], "service", vars["service"], "cluster", url.QueryEscape(body.ClusterID))
	if err != nil {
		fmt.Printf("GET cluster failed %s/%s/%s: %s\n", vars["account"], vars["service"], body.ClusterID, err)
		util.RespondError(w, err)
		return
	}

	service, err = db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err != nil {
		fmt.Printf("POST cluster failed %s/%s/%s %#v\n", vars["account"], vars["service"], body.ClusterID, err)
		util.RespondNotFound(w, err)
	}

	// Check if the LB already has this cluster and error if it does
	if err := service.Service.AddUpstream(body.ClusterID); err != nil {
		fmt.Printf("Duplicate cluster %#v: %s\n", body.ClusterID, err)

		// The LB already had that cluster
		util.RespondConflict(
			w,
			map[string]interface{}{"message": err.Error(), "link": model.Links{"self": selfURL.String()}},
			map[string]string{"Location": selfURL.String()},
		)
		return
	}

	// Prepare the patch. Start with an empty patch and add operations
	// to it.
	patch := []map[string]interface{}{}

	// If this is the first cluster that we're adding to this LB then
	// we need to initialize Spec.UpstreamClusters with an empty array
	// first
	if len(service.Service.Spec.UpstreamClusters) == 1 {
		patch = append(patch, map[string]interface{}{
			"op":    "add",
			"path":  "/spec/upstream-clusters",
			"value": []string{},
		})
	}

	// Add the cluster to the upstream-clusters array
	patch = append(patch, map[string]interface{}{
		"op":    "add",
		"path":  "/spec/upstream-clusters/-",
		"value": body.ClusterID,
	})

	// apply the patch
	if patchBytes, err = json.Marshal(patch); err != nil {
		fmt.Printf("POST cluster failed %#v\n", err)
		util.RespondError(w, err)
		return
	}
	if err = g.client.Patch(r.Context(), &service.Service, client.RawPatch(types.JSONPatchType, patchBytes)); err != nil {
		fmt.Println(string(patchBytes))
		fmt.Printf("POST cluster failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	if err != nil {
		// Something went wrong
		fmt.Printf("POST cluster failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST cluster OK %s/%s %s\n", vars["account"], vars["service"], body.ClusterID)
	http.Redirect(w, r, selfURL.String(), http.StatusFound)
}

func (g *EPIC) showCluster(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		service *model.Service
	)
	vars := mux.Vars(r)

	cluster, err := url.QueryUnescape(vars["cluster"])
	if err != nil {
		fmt.Printf("GET cluster failed %s/%s %s %#v\n", vars["account"], vars["service"], cluster, err)
		util.RespondBad(w, err)
		return
	}

	// 404 if we can't find the service
	service, err = db.ReadService(r.Context(), g.client, vars["account"], vars["service"])
	if err != nil {
		fmt.Printf("GET cluster failed %s/%s %s %#v\n", vars["account"], vars["service"], cluster, err)
		util.RespondNotFound(w, err)
		return
	}

	// 404 if the service doesn't have a cluster with that name
	if !service.Service.ContainsUpstream(cluster) {
		err = fmt.Errorf("cluster %s/%s %s not found", vars["account"], vars["service"], cluster)
		fmt.Printf("GET cluster failed %#v\n", err)
		util.RespondNotFound(w, err)
		return
	}

	srvLink, err := g.router.Get("service").URL("account", vars["account"], "service", vars["service"])
	if err != nil {
		fmt.Printf("GET group failed %s/%s: %s\n", vars["account"], vars["group"], err)
		util.RespondError(w, err)
		return
	}
	links := model.Links{"self": r.RequestURI, "service": srvLink.String()}

	fmt.Printf("GET cluster OK %s/%s %s\n", vars["account"], vars["service"], cluster)
	util.RespondJSON(w, http.StatusOK, model.Cluster{Links: links}, util.EmptyHeader)
	return
}

func (g *EPIC) deleteCluster(w http.ResponseWriter, r *http.Request) {
	var (
		err error
	)
	vars := mux.Vars(r)

	cluster, err := url.QueryUnescape(vars["cluster"])
	if err != nil {
		fmt.Printf("DELETE cluster failed %s/%s %s %#v\n", vars["account"], vars["service"], cluster, err)
		util.RespondBad(w, err)
	}

	if err = db.DeleteCluster(r.Context(), g.client, vars["account"], vars["service"], cluster); err != nil {
		fmt.Printf("DELETE cluster failed %s/%s %s %#v\n", vars["account"], vars["service"], cluster, err)
		util.RespondError(w, err)
	}

	if err = db.DeleteClusterReps(r.Context(), g.client, vars["account"], vars["service"], cluster); err != nil {
		fmt.Printf("DELETE cluster failed %s/%s %s %#v\n", vars["account"], vars["service"], cluster, err)
		util.RespondError(w, err)
	}

	fmt.Printf("DELETE cluster OK %s/%s %s\n", vars["account"], vars["service"], cluster)
	util.RespondJSON(w, http.StatusOK, map[string]string{"message": "delete successful"}, map[string]string{})
	return
}

func (g *EPIC) createServiceEndpoint(w http.ResponseWriter, r *http.Request) {
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

	// Tie the endpoint to the service and cluster. We use a label so we
	// can query for the set of endpoints that belong to a given
	// LB/cluster.
	body.Endpoint.Labels = map[string]string{
		epicv1.OwningLoadBalancerLabel: service.Service.Name,
		epicv1.OwningClusterLabel:      body.Endpoint.Spec.Cluster,
	}

	// Give the endpoint a name that's readable but also won't collide
	// with others
	if body.Endpoint.Name == "" {
		addr := net.ParseIP(body.Endpoint.Spec.Address)
		if addr == nil {
			util.RespondBad(w, fmt.Errorf("%s can't be parsed as a valid IP address", body.Endpoint.Spec.Address))
			return
		}
		body.Endpoint.Name = epicv1.RemoteEndpointName(addr, body.Endpoint.Spec.Port.Port, body.Endpoint.Spec.Port.Protocol)
	}

	// This endpoint will live in the same NS as its owning LB
	body.Endpoint.Namespace = service.Service.Namespace

	// Create the endpoint
	err = g.client.Create(r.Context(), &body.Endpoint)
	if err != nil {
		matches := duplicateRep.FindStringSubmatch(err.Error())
		if len(matches) > 0 {

			// We already had that endpoint, but we can return what we hope
			// the client needs to set up the tunnels on its end
			otherURL := fmt.Sprintf("%s/%s", r.RequestURI, matches[1])
			fmt.Printf("POST endpoint 409/duplicate %s\n", body.Endpoint.Name)
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

	selfURL, err := g.router.Get("endpoint").URL("account", vars["account"], "service", vars["service"], "endpoint", body.Endpoint.Name)
	if err != nil {
		fmt.Printf("POST endpoint failed %s/%s/%s: %s\n", vars["account"], vars["service"], body.Endpoint.Name, err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST endpoint OK %#v\n", body.Endpoint.Spec)
	http.Redirect(w, r, selfURL.String(), http.StatusFound)
	return
}

func (g *EPIC) showEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ep, err := db.ReadEndpoint(r.Context(), g.client, vars["account"], vars["endpoint"])
	if err == nil {
		srvLink, err := g.router.Get("service").URL("account", vars["account"], "service", vars["service"])
		if err != nil {
			fmt.Printf("GET group failed %s/%s: %s\n", vars["account"], vars["group"], err)
			util.RespondError(w, err)
			return
		}
		ep.Links = model.Links{"self": r.RequestURI, "service": srvLink.String()}

		fmt.Printf("GET endpoint OK %s/%s\n", vars["account"], vars["endpoint"])
		util.RespondJSON(w, http.StatusOK, ep, util.EmptyHeader)
		return
	}
	fmt.Printf("GET endpoint failed %s/%s %#v\n", vars["account"], vars["endpoint"], err)
	util.RespondNotFound(w, err)
}

func (g *EPIC) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := db.DeleteEndpoint(r.Context(), g.client, vars["account"], vars["endpoint"])
	if err == nil {
		fmt.Printf("DELETE endpoint OK %s/%s\n", vars["account"], vars["endpoint"])
		util.RespondJSON(w, http.StatusOK, map[string]string{"message": "endpoint deleted"}, util.EmptyHeader)
		return
	}
	fmt.Printf("DELETE endpoint failed %s/%s %#v\n", vars["account"], vars["endpoint"], err)
	util.RespondError(w, err)
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
		srvLink, err := g.router.Get("group-services").URL("account", vars["account"], "group", vars["group"])
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
			"self":           r.RequestURI,
			"account":        acctLink.String(),
			"create-service": srvLink.String(),
			"create-proxy":   proxyLink.String(),
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
	router.HandleFunc("/accounts/{account}/services/{service}/endpoints/{endpoint}", epic.showEndpoint).Methods(http.MethodGet).Name("endpoint")
	router.HandleFunc("/accounts/{account}/services/{service}/endpoints/{endpoint}", epic.deleteEndpoint).Methods(http.MethodDelete)
	router.HandleFunc("/accounts/{account}/services/{service}/endpoints", epic.createServiceEndpoint).Methods(http.MethodPost)

	router.HandleFunc("/accounts/{account}/services/{service}/clusters/{cluster}", epic.showCluster).Methods(http.MethodGet).Name("cluster")
	router.HandleFunc("/accounts/{account}/services/{service}/clusters/{cluster}", epic.deleteCluster).Methods(http.MethodDelete)
	router.HandleFunc("/accounts/{account}/services/{service}/clusters", epic.createServiceCluster).Methods(http.MethodPost)

	router.HandleFunc("/accounts/{account}/services/{service}", epic.deleteService).Methods(http.MethodDelete)
	router.HandleFunc("/accounts/{account}/services/{service}", epic.showService).Methods(http.MethodGet).Name("service")

	router.HandleFunc("/accounts/{account}/groups/{group}/services", epic.createService).Methods(http.MethodPost).Name("group-services")
	router.HandleFunc("/accounts/{account}/groups/{group}", epic.showGroup).Methods(http.MethodGet).Name("group")

	router.HandleFunc("/accounts/{account}", epic.showAccount).Methods(http.MethodGet).Name("account")
}
