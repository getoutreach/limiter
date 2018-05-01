package main

import (
	"flag"
	"time"

	"github.com/getoutreach/limiter/pkg/signals"
	"github.com/golang/glog"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type resourceList struct {
	cpu    string
	memory string
}

type limitRangeItem struct {
	min     resourceList
	request resourceList
	limit   resourceList
	max     resourceList
}
type limitRangeConfig struct {
	container limitRangeItem
	pod       limitRangeItem
}

var (
	kubeconfig   string
	masterURL    string
	config       limitRangeConfig
	numWorkers   int
	syncInterval time.Duration
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

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, syncInterval)
	controller := NewController(kubeClient, kubeInformerFactory)

	go kubeInformerFactory.Start(stopCh)

	if err = controller.Run(numWorkers, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&config.container.limit.cpu, "container.limit.cpu", "", "Default container cpu limit.")
	flag.StringVar(&config.container.limit.memory, "container.limit.memory", "100Mi", "Default container memory limit.")
	flag.StringVar(&config.container.request.cpu, "container.request.cpu", "10m", "Default container cpu request.")
	flag.StringVar(&config.container.request.memory, "container.request.memory", "10Mi", "Default container memory request.")
	flag.StringVar(&config.container.min.cpu, "container.min.cpu", "", "Minimum container cpu request.")
	flag.StringVar(&config.container.min.memory, "container.min.memory", "", "Minimum container memory request.")
	flag.StringVar(&config.container.max.cpu, "container.max.cpu", "", "Maximum container cpu limit.")
	flag.StringVar(&config.container.max.memory, "container.max.memory", "", "Maximum container memory limit.")

	flag.StringVar(&config.pod.min.cpu, "pod.min.cpu", "", "Minimum pod cpu request.")
	flag.StringVar(&config.pod.min.memory, "pod.min.memory", "", "Minimum pod memory request.")
	flag.StringVar(&config.pod.max.cpu, "pod.max.cpu", "", "Maximum pod cpu limit.")
	flag.StringVar(&config.pod.max.memory, "pod.max.memory", "", "Maximum pod memory limit.")

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.IntVar(&numWorkers, "workers", 2, "The number of worker goroutines to spawn.")
	flag.DurationVar(&syncInterval, "interval", time.Second*30, "Sync Loop Interval")
}
