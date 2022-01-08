package app

import (
	"context"
	"fmt"

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
		return nil, fmt.Errorf("error creating k8s session: %w", err)
	}

	return &App{
		k8sSession: k8sSession,
	}, nil
}

func (a *App) queryConnectionSide(ctx context.Context, namespaceName, podName string, portNameOrNum string) (*eval.PodConnection, error) {
	pod, err := a.k8sSession.QueryPod(ctx, namespaceName, podName)
	if err != nil {
		return nil, fmt.Errorf("error querying pod %s %s: %w", namespaceName, podName, err)
	}

	namespace, err := a.k8sSession.QueryNamespace(ctx, namespaceName)
	if err != nil {
		return nil, fmt.Errorf("error querying for namespace %s: %w", namespaceName, err)
	}

	netpolList, err := a.k8sSession.QueryNetPolList(ctx, namespaceName)
	if err != nil {
		return nil, fmt.Errorf("error querying for netpol list %s: %w", namespace, err)
	}

	return eval.NewPodConnection(pod, namespace, netpolList.Items, portNameOrNum)
}

func (a *App) CheckAccess(v ConsoleView,
	namespaceName string,
	podName string,
	toNamespaceName string,
	toPodName string,
	toPortStr string,
	toExternalIP string,
	toProtocolName string) error {

	// UI layer should do user friendly validation. This can just error.
	var err error
	var dest eval.ConnectionSide
	if toPodName != "" {
		dest, err = a.queryConnectionSide(context.TODO(), toNamespaceName, toPodName, toPortStr)
	} else if toExternalIP != "" {
		dest, err = eval.NewExternalConnection(toExternalIP, toPortStr, toProtocolName)
	} else {
		return fmt.Errorf("no destination specified")
	}
	if err != nil {
		return fmt.Errorf("error querying destination: %w", err)
	}

	// TODO parallelize data access
	source, err := a.queryConnectionSide(context.TODO(), namespaceName, podName, "")
	if err != nil {
		return fmt.Errorf("error querying source: %w", err)
	}

	results := eval.Eval(source, dest)
	return RenderCheckAccess(v, results, source, dest)

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
