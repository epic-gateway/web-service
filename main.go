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

	"acnodal.io/epic/web-service/internal/controller"

	epicv1 "epic-gateway.org/resource-model/api/v1"
	// +kubebuilder:scaffold:imports
)

const (
	// URLRoot is the common root of this service's URLs.
	URLRoot = "/api/epic"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	// Seed the RNG so we can generate pseudo-random name suffixes
	rand.Seed(time.Now().UTC().UnixNano())

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(epicv1.AddToScheme(scheme))
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

	// set up web service
	setupLog.Info("starting web service")
	r := mux.NewRouter().UseEncodedPath()
	controller.SetupGWProxyRoutes(r.PathPrefix(URLRoot).Subrouter(), mgr.GetClient())
	controller.SetupGWRouteRoutes(r.PathPrefix(URLRoot).Subrouter(), mgr.GetClient())
	controller.SetupSliceRoutes(r.PathPrefix(URLRoot).Subrouter(), mgr.GetClient())
	controller.SetupEPICRoutes(r.PathPrefix(URLRoot).Subrouter(), mgr.GetClient())
	controller.SetupHealthzRoutes(r.PathPrefix(URLRoot).Subrouter())

	http.Handle("/", r)
	go http.ListenAndServe(":8080", nil)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
