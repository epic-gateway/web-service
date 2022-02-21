package db

import (
	"context"
	"fmt"

	epicv1 "gitlab.com/acnodal/epic/resource-model/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/epic/web-service/internal/model"
)

// ReadSlice reads one endpoint slice from the cluster.
func ReadSlice(ctx context.Context, cl client.Client, accountName string, sliceName string) (*model.Slice, error) {
	mslice := model.NewSlice()
	return &mslice, cl.Get(ctx, client.ObjectKey{Namespace: epicv1.AccountNamespace(accountName), Name: sliceName}, &mslice.Slice)
}

// DeleteSlice deletes the specified endpoint slice.
func DeleteSlice(ctx context.Context, cl client.Client, accountName string, sliceName string) error {
	slice, err := ReadSlice(ctx, cl, accountName, sliceName)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found. Not great, but the client wanted
			// the object gone and it's gone.
			fmt.Printf("%s/%s not found\n", accountName, sliceName)
			return nil
		}
		return err
	}
	return cl.Delete(ctx, &slice.Slice)
}

// UpdateSlice updates the provided endpoint slice.
func UpdateSlice(ctx context.Context, cl client.Client, accountName string, sliceName string, slice *epicv1.GWEndpointSlice) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		model, err := ReadSlice(ctx, cl, accountName, sliceName)
		if err != nil {
			return err
		}

		slice.Spec.DeepCopyInto(&model.Slice.Spec)

		return cl.Update(ctx, &model.Slice)
	})
}
