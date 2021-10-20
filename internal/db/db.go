package db

import (
	"context"
	"fmt"
	"time"

	epicv1 "gitlab.com/acnodal/epic/resource-model/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/util/retry"

	"acnodal.io/epic/web-service/internal/model"
)

// ReadAccount reads one account from the cluster.
func ReadAccount(ctx context.Context, cl client.Client, accountName string) (*model.Account, error) {
	maccount := model.NewAccount()
	return &maccount, cl.Get(ctx, client.ObjectKey{Namespace: epicv1.AccountNamespace(accountName), Name: accountName}, &maccount.Account)
}

// ReadGroup reads one service group from the cluster.
func ReadGroup(ctx context.Context, cl client.Client, accountName string, name string) (*model.Group, error) {
	mgroup := model.NewGroup()
	return &mgroup, cl.Get(ctx, client.ObjectKey{Namespace: epicv1.AccountNamespace(accountName), Name: name}, &mgroup.Group)
}

// ReadService reads one load balancer service from the cluster.
func ReadService(ctx context.Context, cl client.Client, accountName string, name string) (*model.Service, error) {
	var err error
	mservice := model.NewService()
	tries := 2
	for err = fmt.Errorf(""); err != nil && tries > 0; tries-- {
		err = cl.Get(ctx, client.ObjectKey{Namespace: epicv1.AccountNamespace(accountName), Name: name}, &mservice.Service)
		if err != nil {
			fmt.Printf("problem reading service %s/%s: %s\n", accountName, name, err)
			if tries > 1 {
				time.Sleep(1 * time.Second)
			}
		}
	}
	return &mservice, err
}

// ReadEndpoint reads one service endpoint from the cluster.
func ReadEndpoint(ctx context.Context, cl client.Client, accountName string, name string) (*model.Endpoint, error) {
	var err error
	mendpoint := model.NewEndpoint()
	tries := 2
	for err = fmt.Errorf(""); err != nil && tries > 0; tries-- {
		err = cl.Get(ctx, client.ObjectKey{Namespace: epicv1.AccountNamespace(accountName), Name: name}, &mendpoint.Endpoint)
		if err != nil {
			fmt.Printf("problem reading endpoint %s/%s: %#v\n", accountName, name, err)
			if tries > 1 {
				time.Sleep(1 * time.Second)
			}
		}
	}
	return &mendpoint, err
}

// DeleteService deletes the specified load balancer.
func DeleteService(ctx context.Context, cl client.Client, accountName string, name string) error {
	service, err := ReadService(ctx, cl, accountName, name)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found. Not great, but the client wanted
			// the object gone and it's gone.
			fmt.Printf("%s/%s not found. Ignoring since object must be deleted\n", accountName, name)
			return nil
		}
		return err
	}

	// Delete with DeletePropagationForeground policy so endpoints are
	// deleted before the LB. We do this because we need some info from
	// the LB to clean up after the endpoint.
	foreground := v1.DeletePropagationForeground
	return cl.Delete(ctx, &service.Service, &client.DeleteOptions{PropagationPolicy: &foreground})
}

func DeleteCluster(ctx context.Context, cl client.Client, accountName string, serviceName string, cluster string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		service, err := ReadService(ctx, cl, accountName, serviceName)
		if err != nil {
			return err
		}

		if err := service.Service.RemoveUpstream(cluster); err != nil {
			return err
		}

		if err := cl.Update(ctx, &service.Service); err != nil {
			return err
		}

		return nil
	})
}

func DeleteClusterReps(ctx context.Context, cl client.Client, accountName string, serviceName string, cluster string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Delete the endpoints that belong to this cluster
		return cl.DeleteAllOf(
			ctx,
			&epicv1.RemoteEndpoint{},
			client.InNamespace(epicv1.AccountNamespace(accountName)),
			client.MatchingLabels{epicv1.OwningLoadBalancerLabel: serviceName, epicv1.OwningClusterLabel: cluster})
	})
}

// DeleteEndpoint deletes the specified load balancer.
func DeleteEndpoint(ctx context.Context, cl client.Client, accountName string, repName string) error {
	endpoint, err := ReadEndpoint(ctx, cl, accountName, repName)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found. Not great, but the client wanted
			// the object gone and it's gone.
			fmt.Printf("%s/%s not found. Ignoring since object must be deleted\n", accountName, repName)
			return nil
		}
		return err
	}
	return cl.Delete(ctx, &endpoint.Endpoint)
}
