package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
	epicv1 "gitlab.com/acnodal/epic/resource-model/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/epic/web-service/internal/db"
	"acnodal.io/epic/web-service/internal/model"
	"acnodal.io/epic/web-service/internal/util"
)

var (
	duplicateProxy = regexp.MustCompile(`^gwproxies.epic.acnodal.io "(.*)" already exists$`)
)

// GWProxy implements the server side of the GWProxy web service
// protocol.
type GWProxy struct {
	client client.Client
	router *mux.Router
}

// ProxyCreateRequest contains the data from a web service request to
// create a GWProxy.
type ProxyCreateRequest struct {
	Proxy epicv1.GWProxy
}

// createProxy handles PureLB proxy announcements. They're sent from
// the allocator pool, so we need to allocate and return the public
// address.
func (g *GWProxy) create(w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		body ProxyCreateRequest
	)
	vars := mux.Vars(r)

	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		fmt.Printf("POST proxy failed %#v\n", err)
		util.RespondBad(w, err)
		return
	}

	// get the owning group which points to the service prefix from
	// which we'll allocate the address
	group, err := db.ReadGroup(r.Context(), g.client, vars["account"], vars["group"])
	if err != nil {
		fmt.Printf("POST proxy failed %#v\n", err)
		util.RespondNotFound(w, err)
		return
	}

	// Set links to the owning service group and prefix
	if body.Proxy.Labels == nil {
		body.Proxy.Labels = map[string]string{}
	}
	body.Proxy.Labels[epicv1.OwningLBServiceGroupLabel] = vars["group"]
	body.Proxy.Labels[epicv1.OwningServicePrefixLabel] = group.Group.Labels[epicv1.OwningServicePrefixLabel]

	// This proxy will live in the same NS as its owning group and its
	// name will be its client-side UID so it won't collide with other
	// objects
	body.Proxy.Namespace = group.Group.Namespace
	body.Proxy.Name = body.Proxy.Spec.ClientRef.UID
	body.Proxy.Spec.DisplayName = body.Proxy.Spec.ClientRef.Name

	selfURL, err := g.router.Get("proxy").URL("account", vars["account"], "proxy", body.Proxy.Name)
	if err != nil {
		fmt.Printf("POST proxy failed %s/%s/%s: %s\n", vars["account"], vars["group"], body.Proxy.Name, err)
		util.RespondError(w, err)
		return
	}

	// Create the resource
	err = g.client.Create(r.Context(), &body.Proxy)
	if err != nil {
		matches := duplicateProxy.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("POST proxy 409/duplicate %s/%s\n", vars["account"], body.Proxy.Name)

			// We already had that proxy, but we can return what we hope the
			// client needs to set up the tunnels on its end
			util.RespondConflict(
				w,
				map[string]interface{}{"message": err.Error(), "link": model.Links{"self": selfURL.String()}},
				map[string]string{"Location": selfURL.String()},
			)
			return
		}

		// Something else went wrong
		fmt.Printf("POST proxy failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST proxy OK %v %#v\n", vars["account"], body.Proxy.Spec)
	http.Redirect(w, r, selfURL.String(), http.StatusFound)
}

func (g *GWProxy) get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	proxy, err := db.ReadProxy(r.Context(), g.client, vars["account"], vars["proxy"])
	if err == nil {
		groupLink, err := g.router.Get("group").URL("account", vars["account"], "group", proxy.Proxy.Labels[epicv1.OwningLBServiceGroupLabel])
		if err != nil {
			fmt.Printf("GET proxy failed %s/%s: %s\n", vars["account"], vars["group"], err)
			util.RespondError(w, err)
			return
		}
		proxy.Links = model.Links{
			"self":  fmt.Sprintf("%s", r.RequestURI),
			"group": groupLink.String(),
		}
		fmt.Printf("GET proxy OK %s/%s\n", vars["account"], vars["proxy"])
		util.RespondJSON(w, http.StatusOK, proxy, util.EmptyHeader)
		return
	}
	fmt.Printf("GET proxy failed %s/%s %#v\n", vars["account"], vars["proxy"], err)
	util.RespondNotFound(w, err)
}

func (g *GWProxy) del(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Delete the CR
	if err := db.DeleteProxy(r.Context(), g.client, vars["account"], vars["proxy"]); err != nil {
		matches := multiClusterLB.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("service %s has clusters: %s\n", vars["proxy"], err)
			util.RespondConflict(w, map[string]interface{}{"message": err.Error()}, util.EmptyHeader)
			return
		}

		fmt.Printf("DELETE proxy failed %s/%s %#v\n", vars["account"], vars["proxy"], err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("DELETE proxy OK %s/%s\n", vars["account"], vars["proxy"])
	util.RespondJSON(w, http.StatusOK, map[string]string{"message": "delete successful"}, map[string]string{})
	return
}

// SetupGWProxyRoutes sets up the provided mux.Router to handle the web
// service routes.
func SetupGWProxyRoutes(router *mux.Router, client client.Client) {
	proxyCon := &GWProxy{client: client, router: router}
	router.HandleFunc("/accounts/{account}/proxies/{proxy}", proxyCon.del).Methods(http.MethodDelete)
	router.HandleFunc("/accounts/{account}/proxies/{proxy}", proxyCon.get).Methods(http.MethodGet).Name("proxy")
	router.HandleFunc("/accounts/{account}/groups/{group}/proxies", proxyCon.create).Methods(http.MethodPost).Name("group-proxies")
}
