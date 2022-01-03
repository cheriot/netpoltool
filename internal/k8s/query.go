package k8s

import (
	"context"
	"fmt"

	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sSession struct {
	// fyi, Config has max QPS and Burst settings
	config *restclient.Config
	// Share the same http connections for all clients.
	// TODO make private
	Clientset *kubernetes.Clientset
}

func NewSession(kubeconfig string) (*K8sSession, error) {
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sSession{
		config:    config,
		Clientset: clientset,
	}, nil
}

func (s *K8sSession) QueryPod(ctx context.Context, namespace string, podName string) (*corev1.Pod, error) {
	pod, err := s.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, fmt.Errorf("unable to find pod %s in %s", podName, namespace)
	}

	return pod, nil
}

func (s *K8sSession) QueryNetPolList(ctx context.Context, namespace string) (*nwv1.NetworkPolicyList, error) {
	netpolList, err := s.Clientset.NetworkingV1().NetworkPolicies(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error querying for NetworkPolicies %s", err.Error())
	}
	return netpolList, nil
}

func (s *K8sSession) QueryNamespace(ctx context.Context, namespace string) (*corev1.Namespace, error) {
	return s.Clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
}
