package db

import (
	"context"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/egw-ws/internal/model"
)

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

// DeleteService deletes the specified load balancer.
func DeleteService(ctx context.Context, cl client.Client, namespace string, name string) error {
	service, err := ReadService(ctx, cl, namespace, name)
	if err != nil {
		return err
	}
	return cl.Delete(ctx, &service.Service)
}

// CreateService writes a load balancer service to the cluster.
func CreateService(ctx context.Context, cl client.Client, namespace string, service egwv1.LoadBalancer) error {
	service.ObjectMeta.Namespace = "egw-" + namespace
	return cl.Create(ctx, &service)
}

// CreateEndpoint adds a service endpoint to the owning service and
// writes the service back to the cluster.
func CreateEndpoint(ctx context.Context, cl client.Client, namespace string, svcName string, endpoint egwv1.LoadBalancerEndpoint) error {
	var (
		err     error
		service *model.Service
	)

	service, err = ReadService(ctx, cl, namespace, svcName)
	if err == nil {
		service.Service.Spec.Endpoints = append(service.Service.Spec.Endpoints, endpoint)
		return cl.Update(ctx, &service.Service)
	}
	return err
}
