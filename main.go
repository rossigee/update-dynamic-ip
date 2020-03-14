package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

var (
	ip_address_service_url *string
	namespace              *string
	servicename            *string
	kubeconfig             *string
	interval_seconds       = 60
	last_ip_address        = ""
)

func fetch_ip_address(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ipaddr := strings.TrimSpace(string(body))
	trial := net.ParseIP(ipaddr)
	if trial.To4() == nil {
		return "", fmt.Errorf("%v is not an IPv4 address\n", trial)
	}

	return ipaddr, nil
}

func set_ip_address(ipaddrstr string) (bool, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return false, err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, err
	}

	trycount := 0
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		trycount += 1

		// Fetch existing version
		result, getErr := clientset.CoreV1().Services(*namespace).Get(*servicename, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("Failed to get latest version of Service '%s/%s' (try %d): %v", *namespace, *servicename, trycount, getErr)
		}

		// Update with new IP address
		result.Spec.ExternalName = ipaddrstr
		_, updateErr := clientset.CoreV1().Services(*namespace).Update(result)
		if updateErr != nil {
			return fmt.Errorf("Failed to update Service '%s/%s' (try %d): %v", *namespace, *servicename, trycount, updateErr)
		}

		return nil
	})
	if retryErr != nil {
		return false, retryErr
	}

	// Record that we've updated it
	fmt.Printf("Service '%s/%s' updated to point to IP address %s\n", *namespace, *servicename, ipaddrstr)
	last_ip_address = ipaddrstr

	return true, nil
}

func check_status() (bool, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return false, err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, err
	}

	services, err := clientset.CoreV1().Services(*namespace).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	fmt.Printf("There are %d services in the '%s' namespace\n", len(services.Items), *namespace)

	_, err = clientset.CoreV1().Services(*namespace).Get(*servicename, metav1.GetOptions{})
	if err != nil {
		return false, err
	} else if errors.IsNotFound(err) {
		fmt.Printf("Service '%s/%s' not found.\n", *namespace, *servicename)
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		fmt.Printf("Error getting service %v\n", statusError.ErrStatus.Message)
	} else if err != nil {
		return false, err
	}
	fmt.Printf("Service '%s/%s' found.\n", *namespace, *servicename)

	return true, nil
}

func run_at_interval() {
	ip_address, err := fetch_ip_address(*ip_address_service_url)
	if err != nil {
		fmt.Printf("Error getting IP address %v\n", err)
		return
	}

	if last_ip_address == ip_address {
		//fmt.Printf("Same as last time (%s).\n", ip_address)
		return
	}

	fmt.Printf("Setting updated IP '%s'\n", ip_address)
	_, err = set_ip_address(ip_address)
	if err != nil {
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
	_, err = check_status()
	if err != nil {
		panic(err.Error())
	}

	// Main loop
	for true {
		go run_at_interval()
		time.Sleep(time.Duration(interval_seconds) * time.Second)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
