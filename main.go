package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

var (
	namespace  *string
	kubeconfig *string
)

type K8sClient interface {
	GetService(namespace, name string) (*corev1.Service, error)
	UpdateService(service *corev1.Service) error
}

func setIPAddress(client K8sClient, servicename string, ipaddrstr string) error {
	trycount := 0
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		trycount += 1

		// Get the service
		svc, err := client.GetService(*namespace, servicename)
		if err != nil {
			return err
		}
		if svc == nil {
			return fmt.Errorf("failed to get latest version of Service '%s/%s' (try %d): %v", *namespace, servicename, trycount, err)
		}

		// Update with new IP address
		svc.Spec.ExternalName = ipaddrstr
		err = client.UpdateService(svc)
		if err != nil {
			return fmt.Errorf("failed to update Service '%s/%s' (try %d): %v", *namespace, servicename, trycount, err)
		}

		// Record that we've updated it
		log.Infof("Service '%s/%s' updated to point to IP address %s", *namespace, servicename, ipaddrstr)

		return nil
	})
}

func getK8sClient() (K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes clientset: %v", err)
	}

	client := &realK8sClient{clientset: clientset}
	return client, nil
}

type realK8sClient struct {
	clientset *kubernetes.Clientset
}

func (c *realK8sClient) GetService(namespace, name string) (*corev1.Service, error) {
	return c.clientset.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (c *realK8sClient) UpdateService(service *corev1.Service) error {
	_, err := c.clientset.CoreV1().Services(service.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	return err
}

func main() {
	log.Infof("Application starting up")

	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	listen_address := *flag.String("listen-address", "0.0.0.0:8080", "Address to bind to for webhook")
	namespace = flag.String("namespace", "", "Namespace of Service to update")
	flag.Parse()
	log.Infof("Parsed flags: kubeconfig=%s, listen-address=%s, namespace=%s", *kubeconfig, listen_address, *namespace)
	if *namespace == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Initialize K8S client interface
	client, err := getK8sClient()
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	// Start webhook service
	log.Infof("Starting webhook server on %s", listen_address)
	startServer(listen_address, client)

	// Main loop
	log.Infof("Entering main loop")
	for {
		time.Sleep(time.Duration(2) * time.Second)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
