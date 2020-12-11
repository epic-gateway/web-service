package db

import (
	"context"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/egw-ws/internal/model"
)

// CreateService writes a load balancer service to the cluster.
func CreateService(ctx context.Context, cl client.Client, namespace string, service egwv1.LoadBalancer) error {
	service.ObjectMeta.Namespace = "egw-" + namespace
	return cl.Create(ctx, &service)
}

// CreateEndpoint writes an endpoint CR to the cluster.
func CreateEndpoint(ctx context.Context, cl client.Client, namespace string, ep egwv1.Endpoint) error {
	ep.ObjectMeta.Namespace = "egw-" + namespace
	return cl.Create(ctx, &ep)
}

// ReadGroup reads one service group from the cluster.
func ReadGroup(ctx context.Context, cl client.Client, namespace string, name string) (*model.Group, error) {
	mgroup := model.NewGroup()
	return &mgroup, cl.Get(ctx, client.ObjectKey{Namespace: "egw-" + namespace, Name: name}, &mgroup.Group)
}

// ReadService reads one load balancer service from the cluster.
func ReadService(ctx context.Context, cl client.Client, namespace string, name string) (*model.Service, error) {
	mservice := model.NewService()
	return &mservice, cl.Get(ctx, client.ObjectKey{Namespace: "egw-" + namespace, Name: name}, &mservice.Service)
}

// ReadEndpoint reads one service endpoint from the cluster.
func ReadEndpoint(ctx context.Context, cl client.Client, namespace string, name string) (*model.Endpoint, error) {
	mendpoint := model.NewEndpoint()
	return &mendpoint, cl.Get(ctx, client.ObjectKey{Namespace: "egw-" + namespace, Name: name}, &mendpoint.Endpoint)
}

// DeleteService deletes the specified load balancer.
func DeleteService(ctx context.Context, cl client.Client, namespace string, name string) error {
	service, err := ReadService(ctx, cl, namespace, name)
	if err != nil {
		return err
	}
	return cl.Delete(ctx, &service.Service)
}

// DeleteEndpoint deletes the specified load balancer.
func DeleteEndpoint(ctx context.Context, cl client.Client, namespace string, name string) error {
	endpoint, err := ReadEndpoint(ctx, cl, namespace, name)
	if err != nil {
		return err
	}
	return cl.Delete(ctx, &endpoint.Endpoint)
}
