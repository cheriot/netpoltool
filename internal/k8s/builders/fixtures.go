package builders

import (
	nwv1 "k8s.io/api/networking/v1"
)

var Namespaces = []*NamespaceBuilder{
	buildFrontEndDev(),
	buildBackEndDev(),

	buildFrontEnd(),
	buildBackEnd(),
}

// Mock resources
// Production namespaces have strict network policies.
// graphql -> product
// Dev namespaces have the same pods, but no network policies.
// graphql-dev -> product-dev -> external ip

var productAName = "product-a"
var graphqlAName = "graphql-a"

func buildFrontEndDev() *NamespaceBuilder {
	ns := NewNamespaceBuilder("front-end-dev")
	ns.NewPodBuilder(graphqlAName).AddPort("http", 8080).AddPort("admin", 4000)
	return ns
}

func buildBackEndDev() *NamespaceBuilder {
	ns := NewNamespaceBuilder("back-end-dev")
	ns.NewPodBuilder(productAName).AddPort("api", 3000).AddPort("admin", 4000)

	return ns
}

func buildFrontEnd() *NamespaceBuilder {
	nsb := NewNamespaceBuilder("front-end")
	pb := nsb.NewPodBuilder("graphql-a").AddPort("http", 3000).AddPort("admin", 4000)

	nsb.NewNetPolBuilder("permit-egress-back-end").
		SelectPod(&pb.Pod, "name").
		AddEgressByLabel("back-end", makePolicyPort(3000), map[string]string{"name": productAName})

	// TODO Allow egress to the IP of an external service
	// TODO allow dns
	return nsb
}

func buildBackEnd() *NamespaceBuilder {
	nsb := NewNamespaceBuilder("back-end")
	pb := nsb.NewPodBuilder(productAName).AddPort("api", 3000).AddPort("admin", 4000)

	nsb.NewNetPolBuilder("permit-ingress-front-end").
		SelectPod(&pb.Pod, "name").
		AddIngressByLabel("front-end", makePolicyPort(3000), map[string]string{"name": graphqlAName})

	defaultDenyBuilder := nsb.NewNetPolBuilder("default-deny-all")
	defaultDenyBuilder.NetworkPolicy.Spec.PolicyTypes = []nwv1.PolicyType{nwv1.PolicyTypeIngress, nwv1.PolicyTypeEgress}

	// TODO All ingress from another product, graphql
	// TODO Allow egress to the IP of an external service
	// TODO allow dns
	return nsb
}
