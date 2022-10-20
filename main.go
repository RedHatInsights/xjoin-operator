/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"github.com/redhatinsights/xjoin-operator/controllers"
	"github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	k8runtime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	k8slog "sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	xjoinv1alpha1 "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	// +kubebuilder:scaffold:imports
	_ "net/http/pprof"
)

var (
	scheme = k8runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(xjoinv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	xjoinLogger := log.NewLogger("main")
	k8slog.SetLogger(xjoinLogger.Logger)
	ctrl.SetLogger(xjoinLogger.Logger)
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8084", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8085", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	namespace, _, err := kubeconfig.Namespace()

	if err != nil {
		k8slog.Log.Error(err, "error loading namespace")
	}
	k8slog.Log.Info("Running in namespace: " + namespace)

	renewDeadline := 60 * time.Second
	leaseDuration := 90 * time.Second

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "222b734b.cloud.redhat.com",
		RenewDeadline:          &renewDeadline,
		LeaseDuration:          &leaseDuration,
		HealthProbeBindAddress: probeAddr,
		Logger:                 k8slog.Log,
	})
	if err != nil {
		k8slog.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = controllers.NewKafkaConnectReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrl.Log.WithName("controllers").WithName("kafkaconnect"),
		mgr.GetEventRecorderFor("kafkaconnect"),
		namespace,
		false,
	).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "KafkaConnect")
		os.Exit(1)
	}

	if err = controllers.NewValidationReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrl.Log.WithName("controllers").WithName("validation"),
		true,
		mgr.GetEventRecorderFor("validation"),
		namespace,
		false,
	).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "Validation")
		os.Exit(1)
	}

	if err = controllers.NewXJoinDataSourcePipelineReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrl.Log.WithName("controllers").WithName("XJoinDataSourcePipeline"),
		mgr.GetEventRecorderFor("xjoindatasourcepipeline"),
		namespace,
		false,
	).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "XJoinDataSourcePipeline")
		os.Exit(1)
	}

	if err = controllers.NewXJoinDataSourceReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrl.Log.WithName("controllers").WithName("XJoinDataSource"),
		mgr.GetEventRecorderFor("xjoindatasource"),
		namespace,
		false,
	).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "XJoinDataSource")
		os.Exit(1)
	}

	if err = controllers.NewXJoinIndexReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrl.Log.WithName("controllers").WithName("XJoinIndex"),
		mgr.GetEventRecorderFor("xjoinindex"),
		namespace,
		false,
	).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "XJoinIndex")
		os.Exit(1)
	}

	if err = controllers.NewXJoinIndexPipelineReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrl.Log.WithName("controllers").WithName("XJoinIndexPipeline"),
		mgr.GetEventRecorderFor("xjoinindexpipeline"),
		namespace,
		false,
	).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "XJoinIndexPipeline")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig()) //used to read logs from validation pod

	if err = controllers.NewXJoinIndexValidatorReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		clientset,
		ctrl.Log.WithName("controllers").WithName("XJoinIndexValidator"),
		mgr.GetEventRecorderFor("xjoinindexvalidator"),
		namespace,
		false,
	).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "XJoinIndexValidator")
		os.Exit(1)
	}

	if err = (&controllers.XJoinPipelineReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("XJoinPipeline"),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("xjoin"),
	}).SetupWithManager(mgr); err != nil {
		k8slog.Log.Error(err, "unable to create controller", "controller", "XJoinPipeline")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	metrics.Init()

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		k8slog.Log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		k8slog.Log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	k8slog.Log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		k8slog.Log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
