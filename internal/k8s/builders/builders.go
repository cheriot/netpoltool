package builders

import (
	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
				Name: name,
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

func (b *NetPolBuilder) Build() (string, string, runtime.Object) {
	return b.Kind, b.Name, &b.NetworkPolicy
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
