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
	duplicateEndpointSlice = regexp.MustCompile(`^gwendpointslices.epic.acnodal.io "(.*)" already exists$`)
)

// SliceController implements the server side of the GWEndpointSlice web service
// protocol.
type SliceController struct {
	client client.Client
	router *mux.Router
}

func (g *SliceController) create(w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		body model.Slice
	)
	urlParams := mux.Vars(r)

	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		fmt.Printf("POST endpointSlice failed %s\n", err)
		util.RespondBad(w, err)
		return
	}

	// Set a link to the owning account.
	if body.Slice.Labels == nil {
		body.Slice.Labels = map[string]string{}
	}
	body.Slice.Labels[epicv1.OwningAccountLabel] = urlParams["account"]

	// Patch the namespace and name. The GWEndpointSlice will live in
	// the account's namespace, and its name will be the EndpointSlice's
	// UID since that's unique.
	body.Slice.Namespace = epicv1.AccountNamespace(urlParams["account"])
	body.Slice.Name = body.Slice.Spec.ClientRef.UID

	selfURL, err := g.router.Get("slice").URL("account", urlParams["account"], "slice", body.Slice.Name)
	if err != nil {
		fmt.Printf("POST endpointSlice failed %s/%s: %s\n", urlParams["account"], body.Slice.Name, err)
		util.RespondError(w, err)
		return
	}

	// Create the resource
	err = g.client.Create(r.Context(), &body.Slice)
	if err != nil {
		matches := duplicateEndpointSlice.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("POST endpointSlice 409/duplicate %s/%s\n", urlParams["account"], body.Slice.Name)

			// We already had that endpointSlice, but we can return what we hope the
			// client needs to set up the tunnels on its end
			util.RespondConflict(
				w,
				map[string]interface{}{"message": err.Error(), "link": model.Links{"self": selfURL.String()}},
				map[string]string{"Location": selfURL.String()},
			)
			return
		}

		// Something else went wrong
		fmt.Printf("POST endpointSlice failed %#v\n", err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("POST endpointSlice OK %v %#v\n", urlParams["account"], body.Slice.Spec)
	http.Redirect(w, r, selfURL.String(), http.StatusFound)
}

func (g *SliceController) show(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	endpointSlice, err := db.ReadSlice(r.Context(), g.client, vars["account"], vars["slice"])
	if err == nil {
		endpointSlice.Slice.ObjectMeta = metav1.ObjectMeta{}
		endpointSlice.Links = model.Links{
			"self": fmt.Sprintf("%s", r.RequestURI),
		}
		fmt.Printf("GET endpointSlice OK %s/%s\n", vars["account"], vars["slice"])
		util.RespondJSON(w, http.StatusOK, endpointSlice, util.EmptyHeader)
		return
	}
	fmt.Printf("GET endpointSlice failed %s/%s %#v\n", vars["account"], vars["slice"], err)
	util.RespondNotFound(w, err)
}

func (g *SliceController) del(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Delete the CR
	if err := db.DeleteSlice(r.Context(), g.client, vars["account"], vars["slice"]); err != nil {
		matches := multiClusterLB.FindStringSubmatch(err.Error())
		if len(matches) > 0 {
			fmt.Printf("service %s has clusters: %s\n", vars["slice"], err)
			util.RespondConflict(w, map[string]interface{}{"message": err.Error()}, util.EmptyHeader)
			return
		}

		fmt.Printf("DELETE endpointSlice failed %s/%s %#v\n", vars["account"], vars["slice"], err)
		util.RespondError(w, err)
		return
	}

	fmt.Printf("DELETE endpointSlice OK %s/%s\n", vars["account"], vars["slice"])
	util.RespondJSON(w, http.StatusOK, map[string]string{"message": "delete successful"}, map[string]string{})
	return
}

// put implements the HTTP PUT method, which updates an existing
// slice.
func (g *SliceController) put(w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		body model.Slice
	)
	urlParams := mux.Vars(r)

	// See if the slice exists, return 404 if not
	_, err = db.ReadSlice(r.Context(), g.client, urlParams["account"], urlParams["slice"])
	if err != nil {
		fmt.Printf("PUT endpointSlice failed %s/%s %#v\n", urlParams["account"], urlParams["slice"], err)
		util.RespondNotFound(w, err)
		return
	}

	// Decode the request body.
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		fmt.Printf("PUT endpointSlice failed %s/%s %s\n", urlParams["account"], urlParams["slice"], err)
		util.RespondBad(w, err)
		return
	}

	// Update the slice.
	err = db.UpdateSlice(r.Context(), g.client, urlParams["account"], urlParams["slice"], &body.Slice)
	if err != nil {
		fmt.Printf("PUT endpointSlice failed %s\n", err)
		util.RespondError(w, err)
		return
	}

	// Redirect back to this slice's GET endpoint.
	selfURL, err := g.router.Get("slice").URL("account", urlParams["account"], "slice", urlParams["slice"])
	if err != nil {
		fmt.Printf("PUT endpointSlice failed %s/%s: %s\n", urlParams["account"], urlParams["slice"], err)
		util.RespondError(w, err)
		return
	}
	fmt.Printf("PUT endpointSlice OK %v %#v\n", urlParams["account"], body.Slice.Spec)
	http.Redirect(w, r, selfURL.String(), http.StatusFound)
	return
}

// SetupSliceRoutes sets up the provided mux.Router to handle the web
// service routes.
func SetupSliceRoutes(router *mux.Router, client client.Client) {
	sliceCtrl := &SliceController{client: client, router: router}
	router.HandleFunc("/accounts/{account}/slices/{slice}", sliceCtrl.del).Methods(http.MethodDelete)
	router.HandleFunc("/accounts/{account}/slices/{slice}", sliceCtrl.show).Methods(http.MethodGet).Name("slice")
	router.HandleFunc("/accounts/{account}/slices/{slice}", sliceCtrl.put).Methods(http.MethodPut)
	router.HandleFunc("/accounts/{account}/slices", sliceCtrl.create).Methods(http.MethodPost).Name("account-slices")
}
