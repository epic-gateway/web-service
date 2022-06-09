package db

import (
	"context"
	"fmt"
	"time"

	epicv1 "gitlab.com/acnodal/epic/resource-model/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// ReadProxy reads one GWProxy resource from the cluster.
func ReadProxy(ctx context.Context, cl client.Client, accountName string, name string) (*model.Proxy, error) {
	var err error
	mproxy := model.NewProxy()
	tries := 2
	for err = fmt.Errorf(""); err != nil && tries > 0; tries-- {
		err = cl.Get(ctx, client.ObjectKey{Namespace: epicv1.AccountNamespace(accountName), Name: name}, &mproxy.Proxy)
		if err != nil {
			fmt.Printf("problem reading proxy %s/%s: %s\n", accountName, name, err)
			if tries > 1 {
				time.Sleep(1 * time.Second)
			}
		}
	}
	return &mproxy, err
}

// DeleteProxy deletes the specified GWProxy.
func DeleteProxy(ctx context.Context, cl client.Client, accountName string, name string) error {
	err := cl.Delete(
		ctx,
		&epicv1.GWProxy{
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
