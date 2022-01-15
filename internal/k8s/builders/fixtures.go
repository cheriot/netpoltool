package builders

var Namespaces = []*NamespaceBuilder{
	buildFrontEndDev(),
	buildBackEndDev(),
}

func buildFrontEndDev() *NamespaceBuilder {
	ns := NewNamespaceBuilder("front-end-dev")
	ns.NewPodBuilder("graphql-a").AddPort("api", 3000)

	return ns
}

func buildBackEndDev() *NamespaceBuilder {
	ns := NewNamespaceBuilder("back-end-dev")
	ns.NewPodBuilder("product-a").AddPort("api", 3000)

	return ns
}
