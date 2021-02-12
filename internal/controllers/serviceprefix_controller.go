package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"acnodal.io/egw-ws/internal/allocator"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
)

// ServicePrefixReconciler reconciles a ServicePrefix object
type ServicePrefixReconciler struct {
	client.Client
	Log       logr.Logger
	Allocator *allocator.Allocator
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=egw.acnodal.io,resources=serviceprefixes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=egw.acnodal.io,resources=serviceprefixes/status,verbs=get;update;patch

// Reconcile takes a Request and makes the system reflect what the
// Request is asking for.
func (r *ServicePrefixReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	result := ctrl.Result{}
	ctx := context.TODO()

	// Read the prefix that caused the event
	sp := egwv1.ServicePrefix{}
	if err := r.Get(ctx, req.NamespacedName, &sp); err != nil {
		return result, err
	}


	// Tell the allocator about the prefix
	if err := r.Allocator.AddPool(sp); err != nil {
		return result, err
	}

	return result, nil
}

// SetupWithManager sets up this controller to work with the mgr.
func (r *ServicePrefixReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&egwv1.ServicePrefix{}).
		Complete(r)
}
