package db

import (
	"context"
	"fmt"
	"time"

	epicv1 "gitlab.com/acnodal/epic/resource-model/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/epic/web-service/internal/model"
)

// ReadRoute reads one service route from the cluster.
func ReadRoute(ctx context.Context, cl client.Client, accountName string, name string) (*model.Route, error) {
	var err error
	mroute := model.NewRoute()
	tries := 2
	for err = fmt.Errorf(""); err != nil && tries > 0; tries-- {
		err = cl.Get(ctx, client.ObjectKey{Namespace: epicv1.AccountNamespace(accountName), Name: name}, &mroute.Route)
		if err != nil {
			fmt.Printf("problem reading route %s/%s: %#v\n", accountName, name, err)
			if tries > 1 {
				time.Sleep(1 * time.Second)
			}
		}
	}
	return &mroute, err
}

// UpdateRoute updates the provided route.
func UpdateRoute(ctx context.Context, cl client.Client, accountName string, routeName string, route *epicv1.GWRoute) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		model, err := ReadRoute(ctx, cl, accountName, routeName)
		if err != nil {
			return err
		}

		route.Spec.DeepCopyInto(&model.Route.Spec)

		return cl.Update(ctx, &model.Route)
	})
}

// DeleteRoute deletes the specified GWRoute.
func DeleteRoute(ctx context.Context, cl client.Client, accountName string, name string) error {
	err := cl.Delete(
		ctx,
		&epicv1.GWRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: epicv1.AccountNamespace(accountName),
				Name:      name,
			},
		},
	)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found. Not great, but the client wanted
			// the object gone and it's gone.
			fmt.Printf("%s/%s not found. Ignoring since object must be deleted\n", accountName, name)
			return nil
		}
		return err
	}

	return nil
}
