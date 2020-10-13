package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
)

// ServicePrefixCallbacks are how this controller notifies the control
// plane of object changes.
type ServicePrefixCallbacks interface {
	ServicePrefixChanged(*egwv1.ServicePrefix) error
}

// ServicePrefixReconciler reconciles a ServicePrefix object
type ServicePrefixReconciler struct {
	client.Client
	Log       logr.Logger
	Callbacks ServicePrefixCallbacks
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=egw.acnodal.io,resources=serviceprefixes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=egw.acnodal.io,resources=serviceprefixes/status,verbs=get;update;patch

// Reconcile takes a Request and makes the system reflect what the
// Request is asking for.
func (r *ServicePrefixReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	result := ctrl.Result{}
	ctx := context.TODO()
	_ = r.Log.WithValues("serviceprefix", req.NamespacedName)

	// read the object that caused the event
	prefix := &egwv1.ServicePrefix{}
	err := r.Get(ctx, req.NamespacedName, prefix)
	if err != nil {
		return result, err
	}

	r.Callbacks.ServicePrefixChanged(prefix)

	return result, nil
}

// SetupWithManager sets up this controller to work with the mgr.
func (r *ServicePrefixReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&egwv1.ServicePrefix{}).
		Complete(r)
}
