package netpoleval

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestEval(t *testing.T) {
	Convey("Absence of NetworkPolicy means Allow", t, func() {
		source := ConnectionSide{
			Pod:       makePod("PodOne", "NamespaceOne", 0),
			Namespace: makeNamespace("NamespaceOne"),
		}
		dest := ConnectionSide{
			Pod:       makePod("PodTwo", "NamespaceTwo", 3000),
			Namespace: makeNamespace("NamespaceTwo"),
		}

		portResults := EvalAllPorts(&source, &dest)

		So(portResults, ShouldResemble, []PortResult{{
			ToPort:         dest.Pod.Spec.Containers[0].Ports[0],
			Egress:         nil,
			Ingress:        nil,
			EgressAllowed:  true,
			IngressAllowed: true,
			Allowed:        true,
		}})
	})

	Convey("Deny all on egress", t, func() {
		egressDeny := NewPolicyBuilder("EgressDenyAll").
			SetNamespace("NamespaceOne").
			SetDenyEgress().
			Build()

		source := ConnectionSide{
			Pod:       makePod("PodOne", "NamespaceOne", 0),
			Namespace: makeNamespace("NamespaceOne"),
			Policies:  []nwv1.NetworkPolicy{*egressDeny},
		}
		dest := ConnectionSide{
			Pod:       makePod("PodTwo", "NamespaceTwo", 3000),
			Namespace: makeNamespace("NamespaceTwo"),
		}

		portResults := EvalAllPorts(&source, &dest)

		So(portResults, ShouldResemble, []PortResult{{
			ToPort: dest.Pod.Spec.Containers[0].Ports[0],
			Egress: []NetpolResult{{
				Netpol:     *egressDeny,
				EvalResult: Deny,
			}},
			Ingress:        nil,
			EgressAllowed:  false,
			IngressAllowed: true,
			Allowed:        false,
		}})
	})

	Convey("Deny all on ingress", t, func() {

		source := ConnectionSide{
			Pod:       makePod("PodOne", "NamespaceOne", 0),
			Namespace: makeNamespace("NamespaceOne"),
		}

		ingressDeny := NewPolicyBuilder("IngressDenyAll").
			SetNamespace("NamespaceTwo").
			SetDenyIngress().
			Build()
		dest := ConnectionSide{
			Pod:       makePod("PodTwo", "NamespaceTwo", 3000),
			Namespace: makeNamespace("NamespaceTwo"),
			Policies:  []nwv1.NetworkPolicy{*ingressDeny},
		}

		portResults := EvalAllPorts(&source, &dest)

		So(portResults, ShouldResemble, []PortResult{{
			ToPort: dest.Pod.Spec.Containers[0].Ports[0],
			Egress: nil,
			Ingress: []NetpolResult{{
				Netpol:     *ingressDeny,
				EvalResult: Deny,
			}},
			EgressAllowed:  true,
			IngressAllowed: false,
			Allowed:        false,
		}})
	})

	Convey("Unrelated policies are ignored.", t, func() {
		sourcePolicyTypeMismatch := NewPolicyBuilder("IngressDenyAll").
			SetNamespace("NamespaceOne").
			SetDenyIngress().
			Build()
		egressLabelMismatch := NewPolicyBuilder("EgressLabelMismatch").
			SetNamespace("NamespaceOne").
			SetPodLabelSelector("name", "doesnotmatch").
			SetDenyEgress().
			Build()

		source := ConnectionSide{
			Pod:       makePod("PodOne", "NamespaceOne", 0),
			Namespace: makeNamespace("NamespaceOne"),
			Policies: []nwv1.NetworkPolicy{
				*sourcePolicyTypeMismatch,
				*egressLabelMismatch,
			},
		}

		destPolicyTypeMismatch := NewPolicyBuilder("EgressDenyAll").
			SetNamespace("NamespaceTwo").
			SetDenyEgress().
			Build()
		ingressLabelMismatch := NewPolicyBuilder("IngressLabelMismatch").
			SetNamespace("NamespaceTwo").
			SetPodLabelSelector("name", "doesnotmatch").
			SetDenyIngress().
			Build()
		dest := ConnectionSide{
			Pod:       makePod("PodTwo", "NamespaceTwo", 3000),
			Namespace: makeNamespace("NamespaceTwo"),
			Policies: []nwv1.NetworkPolicy{
				*destPolicyTypeMismatch,
				*ingressLabelMismatch,
			},
		}

		portResults := EvalAllPorts(&source, &dest)

		So(portResults, ShouldResemble, []PortResult{{
			ToPort: dest.Pod.Spec.Containers[0].Ports[0],
			Egress: []NetpolResult{
				{Netpol: *sourcePolicyTypeMismatch, EvalResult: NoMatch},
				{Netpol: *egressLabelMismatch, EvalResult: NoMatch},
			},
			Ingress: []NetpolResult{
				{Netpol: *destPolicyTypeMismatch, EvalResult: NoMatch},
				{Netpol: *ingressLabelMismatch, EvalResult: NoMatch},
			},
			EgressAllowed:  true,
			IngressAllowed: true,
			Allowed:        true,
		}})
	})

	Convey("Allow only these pods and this port.", t, func() {
		allowPort := 3000
		sourcePod := makePod("PodOne", "NamespaceOne", 0)
		destPod := makePod("PodTwo", "NamespaceTwo", allowPort)
		policyPort := makePolicyPort(corev1.ProtocolTCP, allowPort)

		// Source & Egress policies
		egressDeny := NewPolicyBuilder("EgressDenyAll").
			SetNamespace("NamespaceOne").
			SetDenyEgress().
			Build()
		egressLabelsAllow3000 := NewPolicyBuilder("EgressAllow3000").
			SetNamespace("NamespaceOne").
			SetPodLabelSelector("name", "PodOne").
			SetEgressRules([]nwv1.NetworkPolicyEgressRule{{
				Ports: policyPort,
				To: []nwv1.NetworkPolicyPeer{{
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"name": "PodTwo"}},
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "NamespaceTwo"}},
				}},
			}}).
			Build()
		egressIpBlockAllow3000 := NewPolicyBuilder("EgressIpBlockAllow3000").
			SetNamespace("NamespaceOne").
			SetPodLabelSelector("name", "PodOne").
			SetEgressRules([]nwv1.NetworkPolicyEgressRule{{
				Ports: policyPort,
				To: []nwv1.NetworkPolicyPeer{{
					IPBlock: &nwv1.IPBlock{
						CIDR: destPod.Status.PodIP + "/16", // CIDR that includes the PodIP
					},
				}},
			}}).
			Build()

		source := ConnectionSide{
			Pod:       sourcePod,
			Namespace: makeNamespace("NamespaceOne"),
			Policies: []nwv1.NetworkPolicy{
				*egressDeny,
				*egressLabelsAllow3000,
				*egressIpBlockAllow3000,
			},
		}

		// Dest and Ingress policies
		ingressDeny := NewPolicyBuilder("IngressDenyAll").
			SetNamespace("NamespaceTwo").
			SetDenyIngress().
			Build()
		ingressLabelsAllow3000 := NewPolicyBuilder("IngressLabelsAllow3000").
			SetNamespace("NamespaceTwo").
			SetPodLabelSelector("name", "PodTwo").
			SetIngressRules([]nwv1.NetworkPolicyIngressRule{{
				Ports: policyPort,
				From: []nwv1.NetworkPolicyPeer{{
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"name": "PodOne"}},
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "NamespaceOne"}},
				}},
			}}).
			Build()
		ingressIpBlockAllow3000 := NewPolicyBuilder("IngressIpBlockAllow3000").
			SetNamespace("NamespaceTwo").
			SetPodLabelSelector("name", "PodTwo").
			SetIngressRules([]nwv1.NetworkPolicyIngressRule{{
				Ports: policyPort,
				From: []nwv1.NetworkPolicyPeer{{
					IPBlock: &nwv1.IPBlock{
						CIDR: destPod.Status.PodIP + "/16", // CIDR that includes the PodIP
					},
				}},
			}}).
			Build()
		dest := ConnectionSide{
			Pod:       destPod,
			Namespace: makeNamespace("NamespaceTwo"),
			Policies: []nwv1.NetworkPolicy{
				*ingressDeny,
				*ingressLabelsAllow3000,
				*ingressIpBlockAllow3000,
			},
		}

		portResults := EvalAllPorts(&source, &dest)

		// Sanity check that we've set up the test correctly.
		ipBlockMatch, err := MatchIPBlock(*egressIpBlockAllow3000.Spec.Egress[0].To[0].IPBlock, sourcePod)
		So(err, ShouldBeNil)
		So(ipBlockMatch, ShouldBeTrue)
		So(PortContains(egressLabelsAllow3000.Spec.Egress[0].Ports[0], dest.Pod.Spec.Containers[0].Ports[0]), ShouldBeTrue)
		So(PortContains(ingressLabelsAllow3000.Spec.Ingress[0].Ports[0], dest.Pod.Spec.Containers[0].Ports[0]), ShouldBeTrue)

		So(portResults, ShouldResemble, []PortResult{{
			ToPort: dest.Pod.Spec.Containers[0].Ports[0],
			Egress: []NetpolResult{
				{Netpol: *egressDeny, EvalResult: Deny},
				{Netpol: *egressLabelsAllow3000, EvalResult: Allow},
				{Netpol: *egressIpBlockAllow3000, EvalResult: Allow},
			},
			Ingress: []NetpolResult{
				{Netpol: *ingressDeny, EvalResult: Deny},
				{Netpol: *ingressLabelsAllow3000, EvalResult: Allow},
				{Netpol: *ingressIpBlockAllow3000, EvalResult: Allow},
			},
			EgressAllowed:  true,
			IngressAllowed: true,
			Allowed:        true,
		}})
	})

	Convey("Matching policy that allows something else (implicit deny)", t, func() {
		destPort := 3000
		netpolPort := 3001
		destPod := makePod("PodTwo", "NamespaceTwo", destPort)

		egressLabelsAllowOther := NewPolicyBuilder("IngressLabelsAllow3000").
			SetNamespace("NamespaceOne").
			SetPodLabelSelector("name", "PodOne").
			SetEgressRules([]nwv1.NetworkPolicyEgressRule{{
				Ports: makePolicyPort(corev1.ProtocolTCP, netpolPort),
				To: []nwv1.NetworkPolicyPeer{{
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"name": "PodTwo"}},
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "NamespaceTwo"}},
				}},
			}}).
			Build()
		egressIpBlockAllowOther := NewPolicyBuilder("IngressIpBlockAllow3000").
			SetNamespace("NamespaceOne").
			SetPodLabelSelector("name", "PodOne").
			SetEgressRules([]nwv1.NetworkPolicyEgressRule{{
				Ports: makePolicyPort(corev1.ProtocolTCP, destPort),
				To: []nwv1.NetworkPolicyPeer{{
					IPBlock: &nwv1.IPBlock{
						CIDR:   destPod.Status.PodIP + "/16",
						Except: []string{destPod.Status.PodIP},
					},
				}},
			}}).
			Build()
		source := ConnectionSide{
			Pod:       makePod("PodOne", "NamespaceOne", 0),
			Namespace: makeNamespace("NamespaceOne"),
			Policies: []nwv1.NetworkPolicy{
				*egressLabelsAllowOther,
				*egressIpBlockAllowOther,
			},
		}

		ingressLabelsAllowOther := NewPolicyBuilder("IngressLabelsAllowOther").
			SetNamespace("NamespaceOne").
			SetIngressRules([]nwv1.NetworkPolicyIngressRule{{
				Ports: makePolicyPort(corev1.ProtocolTCP, netpolPort),
				From: []nwv1.NetworkPolicyPeer{{
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"name": "PodOne"}},
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "NamespaceOne"}},
				}},
			}}).
			Build()
		ingressIpBlockAllowOther := NewPolicyBuilder("IngressIpBlockAllowOther").
			SetNamespace("NamespaceOne").
			SetIngressRules([]nwv1.NetworkPolicyIngressRule{{
				Ports: makePolicyPort(corev1.ProtocolTCP, destPort),
				From: []nwv1.NetworkPolicyPeer{{
					IPBlock: &nwv1.IPBlock{
						CIDR:   destPod.Status.PodIP + "/16",
						Except: []string{destPod.Status.PodIP},
					},
				}},
			}}).
			Build()
		dest := ConnectionSide{
			Pod:       destPod,
			Namespace: makeNamespace("NamespaceTwo"),
			Policies: []nwv1.NetworkPolicy{
				*ingressLabelsAllowOther,
				*ingressIpBlockAllowOther,
			},
		}

		portResults := EvalAllPorts(&source, &dest)

		So(portResults, ShouldResemble, []PortResult{{
			ToPort: dest.Pod.Spec.Containers[0].Ports[0],
			Egress: []NetpolResult{
				{Netpol: *egressLabelsAllowOther, EvalResult: Deny},
				{Netpol: *egressIpBlockAllowOther, EvalResult: Deny},
			},
			Ingress: []NetpolResult{
				{Netpol: *ingressLabelsAllowOther, EvalResult: Deny},
				{Netpol: *ingressIpBlockAllowOther, EvalResult: Deny},
			},
			EgressAllowed:  false,
			IngressAllowed: false,
			Allowed:        false,
		}})
	})

	Convey("No ports specified matches all ports.", t, func() {
		allowAllEgress := NewPolicyBuilder("AllowAllEgress").
			SetNamespace("NamespaceOne").
			SetEgressRules([]nwv1.NetworkPolicyEgressRule{{
				Ports: []nwv1.NetworkPolicyPort{}, // match all ports
				To: []nwv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{}, // match all namespaces
				}},
			}}).
			Build()

		source := ConnectionSide{
			Pod:       makePod("PodOne", "NamespaceOne", 0),
			Namespace: makeNamespace("NamespaceOne"),
			Policies: []nwv1.NetworkPolicy{
				*allowAllEgress,
			},
		}
		dest := ConnectionSide{
			Pod:       makePod("PodTwo", "NamespaceTwo", 0),
			Namespace: makeNamespace("NamespaceTwo"),
			Policies:  []nwv1.NetworkPolicy{},
		}

		portResults := EvalAllPorts(&source, &dest)

		So(portResults, ShouldResemble, []PortResult{{
			ToPort: dest.Pod.Spec.Containers[0].Ports[0],
			Egress: []NetpolResult{
				{Netpol: *allowAllEgress, EvalResult: Allow},
			},
			EgressAllowed:  true,
			IngressAllowed: true,
			Allowed:        true,
		}})

	})

	// TODO: Validate that Rules are OR'ed within a single Policy
}

func makePod(name string, namespace string, port int) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"name": name},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "ContainerOne",
				Ports: []corev1.ContainerPort{{
					Name:          "PortOne",
					ContainerPort: int32(port),
					Protocol:      corev1.ProtocolTCP,
				}},
			}},
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.1",
		},
	}
}

func makeNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
	}
}

func makePolicyPort(protocol corev1.Protocol, num int) []nwv1.NetworkPolicyPort {
	port := intstr.FromInt(num)
	return []nwv1.NetworkPolicyPort{{
		Protocol: &protocol,
		Port:     &port,
	}}
}

type PolicyBuilder struct {
	Policy nwv1.NetworkPolicy
}

func NewPolicyBuilder(name string) *PolicyBuilder {
	return &PolicyBuilder{
		Policy: nwv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

func (b *PolicyBuilder) Build() *nwv1.NetworkPolicy {
	return &b.Policy
}

func (b *PolicyBuilder) SetNamespace(namespace string) *PolicyBuilder {
	b.Policy.ObjectMeta.Namespace = namespace
	return b
}

func (b *PolicyBuilder) SetPodLabelSelector(key, value string) *PolicyBuilder {
	b.Policy.Spec.PodSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			key: value,
		},
	}
	return b
}

func (b *PolicyBuilder) SetEgressRules(rules []nwv1.NetworkPolicyEgressRule) *PolicyBuilder {
	b.Policy.Spec.PolicyTypes = append(b.Policy.Spec.PolicyTypes, nwv1.PolicyTypeEgress)
	b.Policy.Spec.Egress = rules
	return b
}

func (b *PolicyBuilder) SetIngressRules(rules []nwv1.NetworkPolicyIngressRule) *PolicyBuilder {
	b.Policy.Spec.PolicyTypes = append(b.Policy.Spec.PolicyTypes, nwv1.PolicyTypeIngress)
	b.Policy.Spec.Ingress = rules
	return b
}

func (b *PolicyBuilder) SetDenyEgress() *PolicyBuilder {
	b.Policy.Spec.PolicyTypes = []nwv1.PolicyType{nwv1.PolicyTypeEgress}
	return b
}

func (b *PolicyBuilder) SetDenyIngress() *PolicyBuilder {
	b.Policy.Spec.PolicyTypes = []nwv1.PolicyType{nwv1.PolicyTypeIngress}
	return b
}
