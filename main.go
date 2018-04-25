package main

import (
	"flag"
	"time"

	"github.com/golang/glog"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/getoutreach/limiter/pkg/signals"
)

// TODO: config struct?
var (
	cpuLimit   string
	cpuRequest string
	kubeconfig string
	masterURL  string
	memLimit   string
	memRequest string
	numWorkers int
)

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second*30)
	controller := NewController(kubeClient, kubeInformerFactory)

	go kubeInformerFactory.Start(stopCh)

	if err = controller.Run(numWorkers, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&numWorkers, "workers", 2, "The number of worker goroutines to spawn.")
	flag.StringVar(&cpuLimit, "cpu-limit", "", "Default container CPU limit.")
	flag.StringVar(&cpuRequest, "cpu-request", "100m", "Default container CPU request.")
	flag.StringVar(&memLimit, "mem-limit", "100Mi", "Default container memory limit.")
	flag.StringVar(&memRequest, "mem-request", "100Mi", "Default container memory request.")
}