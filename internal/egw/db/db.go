package db

import (
	"context"
	"log"
	"net"

  "github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"

	"acnodal.io/egw-ws/internal/model"
)

func ReadGroup(ctx context.Context, db *pgxpool.Pool, id uuid.UUID) (model.Group, error) {
	group := model.NewGroup()
	row := db.QueryRow(ctx, "SELECT name, created, updated FROM groups WHERE id = $1", id)
	err := row.Scan(&group.Name, group.Created, group.Updated)
	if err != nil {
		log.Printf("reading group: %v", err)
		return group, err
	}
	return group, nil
}

func ReadService(ctx context.Context, db *pgxpool.Pool, id uuid.UUID) (model.Service, error) {
	service := model.NewService()
	row := db.QueryRow(ctx, "SELECT group_id, name, address, created, updated FROM services WHERE id = $1", id)
	rawgid := make([]byte, 16)
	rawcidr := net.IP{}
	err := row.Scan(&rawgid, &service.Name, &rawcidr, service.Created, service.Updated)
	if err != nil {
		log.Printf("reading service: %v", err)
		return service, err
	}
	service.Address = rawcidr.String()
	service.GroupID, err = uuid.FromBytes(rawgid)  // FIXME: use pgxtypes mapping?
	if err != nil {
		log.Printf("parsing group UUID: %v", err)
		return service, err
	}
	return service, nil
}

func CreateService(ctx context.Context, db *pgxpool.Pool, service model.Service) (uuid.UUID, error) {
	id := uuid.UUID{}
	row := db.QueryRow(ctx, "INSERT INTO services (group_id, name, address) VALUES ($1, $2, $3) RETURNING id", service.GroupID, service.Name, service.Address)
	err := row.Scan(&id)
	return id, err
}

func CreateEndpoint(ctx context.Context, db *pgxpool.Pool, endpoint model.Endpoint) (uuid.UUID, error) {
	id := uuid.UUID{}
	row := db.QueryRow(ctx, "INSERT INTO endpoints (service_id, address, port) VALUES ($1, $2, $3) RETURNING id", endpoint.ServiceID, endpoint.Address, endpoint.Port)
	err := row.Scan(&id)
	return id, err
}

func ReadEndpoint(ctx context.Context, db *pgxpool.Pool, id uuid.UUID) (model.Endpoint, error) {
	endpoint := model.NewEndpoint()
	row := db.QueryRow(ctx, "SELECT service_id, address, port, created, updated FROM endpoints WHERE id = $1", id)
	rawsrvid := make([]byte, 16)
	rawcidr := net.IP{}
	err := row.Scan(&rawsrvid, &rawcidr, &endpoint.Port, endpoint.Created, endpoint.Updated)
	if err != nil {
		log.Printf("reading endpoint: %v", err)
		return endpoint, err
	}
	endpoint.Address = rawcidr.String()
	endpoint.ServiceID, err = uuid.FromBytes(rawsrvid)  // FIXME: use pgxtypes mapping?
	if err != nil {
		log.Printf("parsing service UUID: %v", err)
		return endpoint, err
	}
	return endpoint, nil
}

func ReadServiceEndpoints(ctx context.Context, db *pgxpool.Pool, serviceid uuid.UUID) ([]model.Endpoint, error) {
	endpoints := []model.Endpoint{}
	rows, _ := db.Query(ctx, "SELECT e.id, e.address, e.port FROM endpoints e WHERE e.service_id = $1", serviceid)
	defer rows.Close()

	for rows.Next() {
		endpoint := model.Endpoint{ServiceID: serviceid}
		rawid := make([]byte, 16)
		rawcidr := net.IP{}
		err := rows.Scan(&rawid, &rawcidr, &endpoint.Port)
		if err != nil {
			log.Printf("reading endpoint: %v", err)
			return endpoints, err
		}
		endpoint.Address = rawcidr.String()
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}
