package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
)

// LoadBalancerReconciler reconciles a LoadBalancer object
type LoadBalancerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=egw.acnodal.io,resources=loadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=egw.acnodal.io,resources=loadbalancers/status,verbs=get;update;patch

// Reconcile takes a Request and makes the system reflect what the
// Request is asking for.
func (r *LoadBalancerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	result := ctrl.Result{}
	ctx := context.Background()
	_ = r.Log.WithValues("loadbalancer", req.NamespacedName)

	// read the object that caused the event
	lb := &egwv1.LoadBalancer{}
	err := r.Get(ctx, req.NamespacedName, lb)
	if err != nil {
		return result, err
	}

	return result, nil
}

// SetupWithManager sets up this controller to work with the mgr.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&egwv1.LoadBalancer{}).
		Complete(r)
}
