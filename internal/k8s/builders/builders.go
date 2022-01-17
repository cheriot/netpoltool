package builders

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/cheriot/netpoltool/internal/util"
)

type K8sBuilder interface {
	Build() (string, string, runtime.Object)
}

type NamespaceBuilder struct {
	corev1.Namespace
	Objects []K8sBuilder
}

func NewNamespaceBuilder(name string) *NamespaceBuilder {
	return &NamespaceBuilder{
		Namespace: corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"name": name},
			},
		},
	}
}

type PodBuilder struct {
	corev1.Pod
}

func (b *PodBuilder) Build() (string, string, runtime.Object) {
	return b.Kind, b.Name, &b.Pod
}

func (b *NamespaceBuilder) NewPodBuilder(podName string) *PodBuilder {
	builder := &PodBuilder{
		Pod: corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: b.Namespace.Name,
				Labels: map[string]string{
					"name": podName,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "cheriot/clitools:latest",
						Env: []corev1.EnvVar{
							{
								Name: "PUBLIC_POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
								},
							},
							{
								Name: "PUBLIC_NAMESPACE_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
								},
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		},
	}
	b.Objects = append(b.Objects, builder)
	return builder
}

func (b *PodBuilder) AddPort(name string, num int32) *PodBuilder {
	b.Pod.Spec.Containers[0].Ports = append(b.Pod.Spec.Containers[0].Ports, corev1.ContainerPort{Name: name, ContainerPort: num, Protocol: corev1.ProtocolTCP})
	return b
}

type NetPolBuilder struct {
	nwv1.NetworkPolicy
}

func (b *NamespaceBuilder) NewNetPolBuilder(policyName string) *NetPolBuilder {
	builder := &NetPolBuilder{
		NetworkPolicy: nwv1.NetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking.k8s.io/v1",
				Kind:       "NetworkPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: b.Namespace.Name,
			},
		},
	}
	b.Objects = append(b.Objects, builder)
	return builder
}

func (b *NetPolBuilder) Build() (string, string, runtime.Object) {
	return b.Kind, b.Name, &b.NetworkPolicy
}

func (b *NetPolBuilder) SelectPod(pod *corev1.Pod, labels ...string) *NetPolBuilder {
	b.Spec.PodSelector = metav1.LabelSelector{
		MatchLabels: labelsFromPod(pod, labels...),
	}

	return b
}

func (b *NetPolBuilder) AddEgressByLabel(namespace string, toPort nwv1.NetworkPolicyPort, labels map[string]string) *NetPolBuilder {
	if !util.Contains(b.NetworkPolicy.Spec.PolicyTypes, nwv1.PolicyTypeEgress) {
		b.NetworkPolicy.Spec.PolicyTypes = append(b.NetworkPolicy.Spec.PolicyTypes, nwv1.PolicyTypeEgress)
	}
	rule := nwv1.NetworkPolicyEgressRule{
		Ports: []nwv1.NetworkPolicyPort{toPort},
		To:    []nwv1.NetworkPolicyPeer{makePolicyPeer(namespace, labels)},
	}
	b.NetworkPolicy.Spec.Egress = append(b.NetworkPolicy.Spec.Egress, rule)
	return b
}

func (b *NetPolBuilder) AddIngressByLabel(namespace string, toPort nwv1.NetworkPolicyPort, labels map[string]string) *NetPolBuilder {
	if !util.Contains(b.NetworkPolicy.Spec.PolicyTypes, nwv1.PolicyTypeIngress) {
		b.NetworkPolicy.Spec.PolicyTypes = append(b.NetworkPolicy.Spec.PolicyTypes, nwv1.PolicyTypeIngress)
	}
	rule := nwv1.NetworkPolicyIngressRule{
		Ports: []nwv1.NetworkPolicyPort{toPort},
		From:  []nwv1.NetworkPolicyPeer{makePolicyPeer(namespace, labels)},
	}
	b.NetworkPolicy.Spec.Ingress = append(b.NetworkPolicy.Spec.Ingress, rule)
	return b
}

func makePolicyPeer(namespace string, labels map[string]string) nwv1.NetworkPolicyPeer {
	return nwv1.NetworkPolicyPeer{
		PodSelector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"name": namespace},
		},
	}
}

func makePolicyPort(port int) nwv1.NetworkPolicyPort {
	protocol := corev1.ProtocolTCP
	structPort := intstr.FromInt(port)
	return nwv1.NetworkPolicyPort{
		Protocol: &protocol,
		Port:     &structPort,
	}
}

func labelsFromPod(pod *corev1.Pod, labels ...string) map[string]string {
	matchLabels := map[string]string{}
	for _, l := range labels {
		val, ok := pod.Labels[l]
		if !ok {
			panic(fmt.Sprintf("pod %s does not have label %s among labels %v", pod.Name, l, pod.Labels))
		}
		matchLabels[l] = val
	}
	return matchLabels
}
