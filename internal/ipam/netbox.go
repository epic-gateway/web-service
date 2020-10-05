package ipam

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"

	"acnodal.io/egw-ws/internal/util"
)

// Address represents one IP address.
type Address struct {
	ID      int    `json:"id"`
	Address string `json:"address"`
}

// ListAddressesResponse contains the response to a ListAddresses
// request.
type ListAddressesResponse struct {
	Count   int       `json:"count"`
	Results []Address `json:"results"`
}

// IPAM manages IP addresses.
type IPAM struct {
	db *pgxpool.Pool
}

func (ipam *IPAM) queryAddresses(ctx context.Context, tenant string, status string) ([]Address, error) {
	addrs := make([]Address, 0)
	rows, err := ipam.db.Query(ctx, "select i.id, i.address from ipam_ipaddress i inner join tenancy_tenant t on i.tenant_id = t.id where t.slug = $1 and i.status = $2", tenant, status)
	if err != nil {
		return addrs, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var address net.IP
		err = rows.Scan(&id, &address)
		if err != nil {
			return addrs, err
		}
		addrs = append(addrs, Address{ID: id, Address: address.String() + "/32"})
	}
	if rows.Err() != nil {
		return addrs, rows.Err()
	}

	return addrs, nil
}

func (ipam *IPAM) listAddresses(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	addresses, err := ipam.queryAddresses(ctx, r.FormValue("tenant"), r.FormValue("status"))
	if err != nil {
		log.Println(err)
		util.RespondError(w)
	} else {
		util.RespondJSON(w, ListAddressesResponse{Count: len(addresses), Results: addresses})
	}
}

func (ipam *IPAM) updateAddress(ctx context.Context, id int, status string) error {
	_, err := ipam.db.Exec(ctx, `UPDATE ipam_ipaddress set status = $1 where id = $2`, status, id)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (ipam *IPAM) patchAddress(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		log.Println("id not provided")
	} else {
		params := map[string]string{}
		err = json.NewDecoder(r.Body).Decode(&params)
		if err != nil {
			log.Println("can't parse input")
		} else {
			newStatus := params["status"]
			if newStatus == "" {
				log.Println("status not provided")
			} else {
				err := ipam.updateAddress(ctx, id, newStatus)
				if err != nil {
					log.Println(err)
					util.RespondError(w)
					return
				}

				w.WriteHeader(http.StatusOK)
				return
			}
		}
	}
	util.RespondBad(w)
}

// NewIPAM configures a new IPAM instance.
func NewIPAM(pool *pgxpool.Pool) *IPAM {
	return &IPAM{db: pool}
}

// SetupRoutes sets up the provided mux.Router to handle the IPAM web
// service routes.
func SetupRoutes(router *mux.Router, prefix string, pool *pgxpool.Pool) {
	ipam := NewIPAM(pool)
	sr := router.PathPrefix(prefix).Subrouter()
	sr.HandleFunc("/ip-addresses/", ipam.listAddresses).Methods(http.MethodGet)
	sr.HandleFunc("/ip-addresses/{id:[0-9]+}/", ipam.patchAddress).Methods(http.MethodPatch)
}
