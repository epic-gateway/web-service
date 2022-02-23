package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
	epicv1 "gitlab.com/acnodal/epic/resource-model/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/epic/web-service/internal/db"
	"acnodal.io/epic/web-service/internal/model"
	"acnodal.io/epic/web-service/internal/util"
)

var (
	duplicateRoute = regexp.MustCompile(`^gwroutes.epic.acnodal.io "(.*)" already exists$`)
)

// GWRoute implements the server side of the GWRoute web service
// protocol.
type GWRoute struct {
	client client.Client
	router *mux.Router
}

// RouteCreateRequest contains the data from a web service request to
// create a Route.
type RouteCreateRequest struct {
	Route epicv1.GWRoute
}

func (g *GWRoute) create(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var (
		body RouteCreateRequest
		err  error
	)

	// Parse request
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.RespondBad(w, err)
		return
	}

	// Patch the route namespace and name. The GWRoute will live in the
	// account's namespace, and its name will be the HTTPRoute's UID
	// since that's unique.
	body.Route.Namespace = epicv1.AccountNamespace(vars["account"])
	body.Route.Name = body.Route.Spec.ClientRef.UID

	selfURL, err := g.router.Get("route").URL("account", vars["account"], "route", body.Route.Name)
	if err != nil {
		fmt.Printf("POST route failed %s/%s/%s: %s\n", vars["account"], vars["service"], body.Route.Name, err)
		util.RespondError(w, err)
		return
	}

	// Create the route
	if err := g.client.Create(r.Context(), &body.Route); err != nil {
		matches := duplicateRoute.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("POST route 409/duplicate %s/%s\n", vars["account"], body.Route.Name)

			// We already had that route, but we can return what we hope the
			// client needs.
			util.RespondConflict(
				w,
				map[string]interface{}{"message": err.Error(), "link": model.Links{"self": selfURL.String()}},
				map[string]string{"Location": selfURL.String()},
			)
			return
		}

		// Something else went wrong
		fmt.Printf("POST route failed %#v %#v\n", body, err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST route OK %#v\n", body.Route.Spec)
	http.Redirect(w, r, selfURL.String(), http.StatusFound)
	return
}

func (g *GWRoute) show(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	route, err := db.ReadRoute(r.Context(), g.client, vars["account"], vars["route"])
	if err == nil {
		route.Links = model.Links{
			"self": fmt.Sprintf("%s", r.RequestURI),
		}
		route.Route.ObjectMeta = metav1.ObjectMeta{}

		fmt.Printf("GET route OK %s/%s\n", vars["account"], vars["route"])
		util.RespondJSON(w, http.StatusOK, route, util.EmptyHeader)
		return
	}
	fmt.Printf("GET route failed %s/%s %#v\n", vars["account"], vars["route"], err)
	util.RespondNotFound(w, err)
}

func (g *GWRoute) del(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	err := db.DeleteRoute(r.Context(), g.client, vars["account"], vars["route"])
	if err == nil {
		fmt.Printf("DELETE route OK %s/%s\n", vars["account"], vars["route"])
		util.RespondJSON(w, http.StatusOK, map[string]string{"message": "route deleted"}, util.EmptyHeader)
		return
	}
	fmt.Printf("DELETE route failed %s/%s %#v\n", vars["account"], vars["route"], err)
	util.RespondError(w, err)
}

// SetupEPICRoutes sets up the provided mux.Router to handle the web
// service routes.
func SetupGWRouteRoutes(router *mux.Router, client client.Client) {
	routeCon := &GWRoute{client: client, router: router}
	router.HandleFunc("/accounts/{account}/routes/{route}", routeCon.show).Methods(http.MethodGet).Name("route")
	router.HandleFunc("/accounts/{account}/routes/{route}", routeCon.del).Methods(http.MethodDelete)
	router.HandleFunc("/accounts/{account}/routes", routeCon.create).Methods(http.MethodPost).Name("account-routes")
}
