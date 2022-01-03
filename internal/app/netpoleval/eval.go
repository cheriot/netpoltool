package netpoleval

import (
	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"

	"github.com/cheriot/netpoltool/internal/util"
)

type PortResult struct {
	ToPort         corev1.ContainerPort
	Egress         []NetpolResult
	Ingress        []NetpolResult
	IngressAllowed bool
	EgressAllowed  bool
	Allowed        bool
}

type NetpolResult struct {
	Netpol nwv1.NetworkPolicy
	EvalResult
}

type EvalResult uint8

const (
	NoMatch EvalResult = iota
	Deny
	Allow
)

func EvalResultString(er EvalResult) string {
	return []string{"NoMatch", "Deny", "Allow"}[er]
}

func EvalAllPorts(source, dest *ConnectionSide) []PortResult {
	return Eval(source, dest, dest.GetContainerPorts())
}

func Eval(source, dest *ConnectionSide, toPorts []corev1.ContainerPort) []PortResult {
	util.Log.Debugf("Eval toPorts %+v", toPorts)

	var portResults []PortResult
	for _, toPort := range toPorts {
		var egressResults []NetpolResult
		var ingressResults []NetpolResult
		for _, np := range source.Policies {
			egressResults = append(egressResults, NetpolResult{
				EvalResult: evalEgress(*source.Pod, np, dest.Namespace, dest.Pod, toPort),
				Netpol:     np,
			})
		}

		for _, np := range dest.Policies {
			ingressResults = append(ingressResults, NetpolResult{
				EvalResult: evalIngress(dest.Pod, np, source.Namespace, source.Pod, toPort),
				Netpol:     np,
			})
		}

		egressAllowed := combineNetpolResults(egressResults)
		ingressAllowed := combineNetpolResults(ingressResults)
		portResults = append(portResults, PortResult{
			ToPort:         toPort,
			Egress:         egressResults,
			Ingress:        ingressResults,
			EgressAllowed:  egressAllowed,
			IngressAllowed: ingressAllowed,
			Allowed:        egressAllowed && ingressAllowed,
		})

	}

	return portResults
}

func combineNetpolResults(nrs []NetpolResult) bool {
	ers := util.Map(nrs, func(nr NetpolResult) EvalResult { return nr.EvalResult })

	// NoMatch, Deny  -> Deny
	// NoMatch, Allow -> Allow
	// Deny, Allow    -> Allow
	max := util.Fold(ers, NoMatch, util.Max[EvalResult])

	// The absence of a policy is Allow
	if max == NoMatch {
		return true
	}
	return max == Allow
}

func evalIngress(
	toPod *corev1.Pod,
	netpol nwv1.NetworkPolicy,
	sourceNamespace *corev1.Namespace,
	sourcePod *corev1.Pod,
	toPort corev1.ContainerPort) EvalResult {

	util.Log.Debugf("Eval ingress policy %s %s from pod %s %s on port %s %d", toPod.Namespace, netpol.Name, sourcePod.Namespace, sourcePod.Name, toPort.Name, toPort.ContainerPort)

	if !util.Contains(netpol.Spec.PolicyTypes, nwv1.PolicyTypeIngress) {
		// netpol does not describe ingress
		util.Log.Tracef("Policy does not describe ingress %s %s", netpol.Namespace, netpol.Name)
		return NoMatch
	}

	if !MatchLabelSelector(netpol.Spec.PodSelector, toPod.Labels) {
		// netpol does not match source pod
		util.Log.Tracef("Policy does not match pod %+v %+v", netpol.Spec.PodSelector, toPod.Labels)
		return NoMatch
	}

	// does an egress rule match the toPod and toPort?
	for _, eRule := range netpol.Spec.Ingress {
		if evalRule(netpol.Namespace, eRule.From, eRule.Ports, sourceNamespace, sourcePod, toPort) {
			return Allow
		}
	}

	util.Log.Debugf("Ingress denied for lack of a matching rule")
	return Deny
}

func evalEgress(sourcePod corev1.Pod, netpol nwv1.NetworkPolicy, toNamespace *corev1.Namespace, toPod *corev1.Pod, toPort corev1.ContainerPort) EvalResult {
	util.Log.Debugf("Eval egress for policy %s %s to pod %s %s", netpol.Namespace, netpol.Name, toPod.Namespace, toPod.Name)

	if !util.Contains(netpol.Spec.PolicyTypes, nwv1.PolicyTypeEgress) {
		util.Log.Tracef("Policy does not describe egress %s %s", netpol.Namespace, netpol.Name)
		// netpol does not describe egress
		return NoMatch
	}

	if !MatchLabelSelector(netpol.Spec.PodSelector, sourcePod.Labels) {
		// netpol does not match source pod
		util.Log.Tracef("Policy does not match pod %+v %+v", netpol.Spec.PodSelector, sourcePod.Labels)
		return NoMatch
	}

	// does an egress rule match the toPod and toPort?
	for _, eRule := range netpol.Spec.Egress {
		if evalRule(netpol.Namespace, eRule.To, eRule.Ports, toNamespace, toPod, toPort) {
			return Allow
		}
	}

	util.Log.Debugf("Egress denied for lack of a matching rule")
	return Deny
}

func evalRule(
	policyNamespace string,
	peers []nwv1.NetworkPolicyPeer,
	ports []nwv1.NetworkPolicyPort,
	otherNamespace *corev1.Namespace,
	otherPod *corev1.Pod,
	toPort corev1.ContainerPort,
) bool {

	// If any peers match otherPod, compare ports. If both match return true.
	for _, peer := range peers {
		var peerMatch bool
		if peer.IPBlock != nil {
			// "If this field [peer.IPBlock] is set then neither of the other fields can be."
			ipBlockMatch, err := MatchIPBlock(*peer.IPBlock, otherPod)
			if err != nil {
				// Bail because by the time a user knows the podName this shouldn't be possible.
				util.Log.Panicf("error evaluating policy in namespace %s against pod %s %s", policyNamespace, otherPod.Namespace, otherPod.Name)
			}
			util.Log.Tracef("IPBlock compared %t %v %+v", ipBlockMatch, *peer.IPBlock, otherPod.Status.PodIPs)

			peerMatch = ipBlockMatch
		} else {
			// In the absence of peer.IPBlock, NamespaceSelector and PodSelector have meaning when they are nil.

			var namespaceMatch bool
			if peer.NamespaceSelector == nil {
				// "Otherwise it selects the Pods matching PodSelector in the policy's own Namespace"
				namespaceMatch = otherNamespace.Name == policyNamespace
			} else {
				namespaceMatch = MatchLabelSelector(*peer.NamespaceSelector, otherNamespace.GetLabels())
			}

			var podMatch bool
			if peer.PodSelector == nil {
				// "if present but empty, it selects all pods"
				podMatch = true
			} else {
				podMatch = MatchLabelSelector(*peer.PodSelector, otherPod.GetLabels())
			}

			util.Log.Tracef("Comparing peer selectors %v %v to pod labels %v", peer.NamespaceSelector, peer.PodSelector, otherPod.GetLabels())
			util.Log.Debugf("Namespace and pod selectors compared: %t %t", namespaceMatch, podMatch)

			peerMatch = namespaceMatch && podMatch
		}

		if peerMatch {
			for _, policyPort := range ports {
				if PortContains(policyPort, toPort) {
					util.Log.Debugf("Peer and port match %+v applys to %+v", policyPort, toPort)
					return true
				}
			}
			util.Log.Debugf("Peer match, but port not found in policy %+v", toPort)
			return false
		}
	}

	util.Log.Tracef("evalRule did not match peers %+v on %s %s", peers, otherNamespace.Name, otherPod.Name)
	return false
}
