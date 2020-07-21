package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"

	"acnodal.io/egw/web/egw"
	"acnodal.io/egw/web/ipam"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	r := mux.NewRouter()

	ipam.SetupRoutes(r, "/api/ipam", pool)
	egw.SetupRoutes(r, "/api/egw")

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
