package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"

	"acnodal.io/egw-ws/internal/egw"
	"acnodal.io/egw-ws/internal/envoy"
	"acnodal.io/egw-ws/internal/ipam"
)

var (
	wsPort uint

	xDSDebug bool
	xDSPort  uint
	nodeID   string
)

func init() {
	flag.BoolVar(&xDSDebug, "debug", false, "Enable xds debug logging")
	flag.UintVar(&wsPort, "ws-port", 8080, "Web service port")
	flag.UintVar(&xDSPort, "xds-port", 18000, "xDS management server port")
	flag.StringVar(&nodeID, "nodeID", "egw-cp", "Envoy node ID")
}

func main() {
	flag.Parse()

	// initialize the database connection pool
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// launch the Envoy xDS control plane in the background
	go envoy.LaunchControlPlane(xDSPort, nodeID, xDSDebug)

	// run the EGW web service in the foreground
	r := mux.NewRouter()
	ipam.SetupRoutes(r, "/api/ipam", pool)
	egw.SetupRoutes(r, "/api/egw", pool)
	http.Handle("/", r)
	port := fmt.Sprintf(":%d", wsPort)
	log.Printf("web service listening on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
