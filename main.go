package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

var (
	ip_address_service_url *string
	namespace              *string
	servicename            *string
	kubeconfig             *string
	last_ip_address        = ""
)

func fetchIPAddress(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}
		bodyBytes = append(bodyBytes, buf[:n]...)
	}

	ipaddr := strings.TrimSpace(string(bodyBytes))
	trial := net.ParseIP(ipaddr)
	if trial.To4() == nil {
		return "", fmt.Errorf("%v is not an IPv4 address", trial)
	}

	return ipaddr, nil
}

func setIPAddress(ipaddrstr string) error {
	config, err := getK8SConfig()
	if err != nil {
		return err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	trycount := 0
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		trycount += 1
		servicesApi := clientset.CoreV1().Services(*namespace)

		// Fetch existing version
		result, getErr := servicesApi.Get(context.TODO(), *servicename, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("failed to get latest version of Service '%s/%s' (try %d): %v", *namespace, *servicename, trycount, getErr)
		}

		// Update with new IP address
		result.Spec.ExternalName = ipaddrstr
		_, updateErr := servicesApi.Update(context.TODO(), result, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to update Service '%s/%s' (try %d): %v", *namespace, *servicename, trycount, updateErr)
		}

		return nil
	})
	if retryErr != nil {
		return retryErr
	}

	// Record that we've updated it
	log.Infof("Service '%s/%s' updated to point to IP address %s\n", *namespace, *servicename, ipaddrstr)
	last_ip_address = ipaddrstr

	return nil
}

func getK8SConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return config, nil
}

func checkStatus() (bool, error) {
	config, err := getK8SConfig()
	if err != nil {
		return false, err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, err
	}

	servicesApi := clientset.CoreV1().Services(*namespace)
	services, err := servicesApi.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	log.Debugf("There are %d services in the '%s' namespace\n", len(services.Items), *namespace)

	_, err = clientset.CoreV1().Services(*namespace).Get(context.TODO(), *servicename, metav1.GetOptions{})
	if err != nil {
		return false, err
	} else if errors.IsNotFound(err) {
		err = fmt.Errorf("service '%s/%s' not found", *namespace, *servicename)
		return false, err
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		err = fmt.Errorf("error getting service: %v", statusError.ErrStatus.Message)
		return false, err
	}
	fmt.Printf("Service '%s/%s' found.\n", *namespace, *servicename)

	return true, nil
}

func runAtInterval() {
	ip_address, err := fetchIPAddress(*ip_address_service_url)
	if err != nil {
		fmt.Printf("Error getting IP address %v\n", err)
		return
	}

	if last_ip_address == ip_address {
		//fmt.Printf("Same as last time (%s).\n", ip_address)
		return
	}

	fmt.Printf("Setting updated IP '%s'\n", ip_address)
	if err = setIPAddress(ip_address); err != nil {
		fmt.Printf("ERROR: Unable to set IP address (%s): %v", ip_address, err)
		return
	}
}

func main() {
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	ip_address_service_url = flag.String("ip-address-service-url", "http://icanhazip.com", "URL of dynamic IP address service to query (optional)")
	interval_seconds := *flag.Int("interval-seconds", 60, "Seconds to wait between checks (0 to disable checks)")
	listen_address := *flag.String("listen-address", "0.0.0.0:8080", "Address to bind to for webhook")
	namespace = flag.String("namespace", "", "Namespace of Service to update")
	servicename = flag.String("servicename", "", "Name of Service to update")
	flag.Parse()
	if *namespace == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *servicename == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Validate provided URL
	_, err := url.Parse(*ip_address_service_url)
	if err != nil {
		panic(err.Error())
	}

	// Check status
	_, err = checkStatus()
	if err != nil {
		panic(err.Error())
	}

	// Start webhook service
	if listen_address != "" {
		startServer(listen_address)
	}

	// Main loop
	for {
		if interval_seconds > 0 {
			go runAtInterval()
		}
		time.Sleep(time.Duration(interval_seconds) * time.Second)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
