package main

import (
	"flag"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"acnodal.io/egw-ws/internal/allocator"
	"acnodal.io/egw-ws/internal/controllers"
	"acnodal.io/egw-ws/internal/egw"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	// Seed the RNG so we can generate pseudo-random name suffixes
	rand.Seed(time.Now().UTC().UnixNano())

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(egwv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":7472", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "1cb3972f.acnodal.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	// set up address allocator
	allocator := allocator.NewAllocator()

	// set up web service
	setupLog.Info("starting web service")
	r := mux.NewRouter()
	egw.SetupRoutes(r, "/api/egw", mgr.GetClient(), allocator)

	http.Handle("/", r)
	go http.ListenAndServe(":8080", nil)

	// launch manager
	if err = (&controllers.ServicePrefixReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("ServicePrefix"),
		Allocator: allocator,
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServicePrefix")
		os.Exit(1)
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
