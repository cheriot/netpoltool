package app

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"

	eval "github.com/cheriot/netpoltool/internal/app/netpoleval"
	"github.com/cheriot/netpoltool/internal/k8s"
)

type App struct {
	k8sSession *k8s.K8sSession
}

func NewApp(kubeconfig string) (*App, error) {
	k8sSession, err := k8s.NewSession(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error creating k8s session <%s>: %w", kubeconfig, err)
	}

	return &App{
		k8sSession: k8sSession,
	}, nil
}

func (a *App) queryConnectionSide(namespaceName, podName string) (*eval.ConnectionSide, error) {
	pod, err := a.k8sSession.QueryPod(context.TODO(), namespaceName, podName)
	if err != nil {
		return nil, fmt.Errorf("error querying pod %s %s: %w", namespaceName, podName, err)
	}

	namespace, err := a.k8sSession.QueryNamespace(context.TODO(), namespaceName)
	if err != nil {
		return nil, fmt.Errorf("error querying for namespace %s: %w", namespaceName, err)
	}

	netpolList, err := a.k8sSession.QueryNetPolList(context.TODO(), namespaceName)
	if err != nil {
		return nil, fmt.Errorf("error querying for netpol list %s: %w", namespace, err)
	}

	return &eval.ConnectionSide{
		Pod:       pod,
		Namespace: namespace,
		Policies:  netpolList.Items,
	}, nil
}

func (a *App) CheckAccess(w io.Writer, namespaceStr string, podName string, toNamespaceStr string, toPodName string, toPortStr string) error {
	// TODO parallelize data access

	source, err := a.queryConnectionSide(namespaceStr, podName)
	if err != nil {
		return fmt.Errorf("error querying source: %w", err)
	}

	dest, err := a.queryConnectionSide(toNamespaceStr, toPodName)
	if err != nil {
		return fmt.Errorf("error querying dest: %w", err)
	}

	var results []eval.PortResult
	if toPortStr != "" {
		port, err := dest.GetPort(toPortStr)
		if err != nil {
			return fmt.Errorf("port name or number is not valid: %w", err)
		}
		results = eval.Eval(source, dest, []corev1.ContainerPort{*port})
	} else {
		results = eval.Eval(source, dest, dest.GetContainerPorts())
	}

	return RenderCheckAccess(w, results, dest)

}

func (a *App) InspectEgress(namespace string, podName string) error {
	pod, err := a.k8sSession.QueryPod(context.TODO(), namespace, podName)
	if err != nil {
		return fmt.Errorf("error from InspectEgress: %w", err)
	}

	netpolList, err := a.k8sSession.QueryNetPolList(context.TODO(), namespace)
	matches := filterMatchingNetpols(netpolList, pod)
	RenderNetPolMatch(matches)

	return nil
}

func filterMatchingNetpols(netpolList *nwv1.NetworkPolicyList, pod *corev1.Pod) []nwv1.NetworkPolicy {
	podLabels := pod.ObjectMeta.Labels
	filteredNetPols := make([]nwv1.NetworkPolicy, 0, len(netpolList.Items))
	for _, np := range netpolList.Items {
		if eval.MatchLabelSelector(np.Spec.PodSelector, podLabels) {
			filteredNetPols = append(filteredNetPols, np)
		}
	}

	return filteredNetPols
}
