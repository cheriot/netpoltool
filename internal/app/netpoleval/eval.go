package netpoleval

import (
	"fmt"
	"os"

	nwv1 "k8s.io/api/networking/v1"

	"github.com/cheriot/netpoltool/internal/util"
)

type PortResult struct {
	ToPort         DestinationPort
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

func Eval(source *PodConnection, dest ConnectionSide) []PortResult {
	util.Log.Debugf("Eval toPorts %+v", dest.GetPorts())

	nodeName := source.Pod.Spec.NodeName
	if nodeName != "" && dest.IsOnNode(nodeName) {
		// "traffic to and from the node where a Pod is running is always allowed, regardless of the IP address of the Pod or the node"
		// https://kubernetes.io/docs/concepts/services-networking/network-policies/
		// That's probably not what the user is interested in so continue evaluation.
		fmt.Fprintf(os.Stderr, "Source and destination are on the same Node, %s, so kubernetes will not evaluate Network Policies and allow access. Evaluation will continue as if this were not the case.", source.Pod.Spec.NodeName)
	}

	var portResults []PortResult
	for _, toPort := range dest.GetPorts() {
		var egressResults []NetpolResult
		var ingressResults []NetpolResult

		if source.IsInCluster() {
			for _, np := range source.GetPolicies() {
				egressResults = append(egressResults, NetpolResult{
					EvalResult: evalEgress(source, np, dest, toPort),
					Netpol:     np,
				})
			}
		}

		if dest.IsInCluster() {
			for _, np := range dest.GetPolicies() {
				ingressResults = append(ingressResults, NetpolResult{
					EvalResult: evalIngress(dest, np, source, toPort),
					Netpol:     np,
				})
			}
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

//func evalPolicy(source)

func evalIngress(
	dest ConnectionSide,
	netpol nwv1.NetworkPolicy,
	source ConnectionSide,
	toPort DestinationPort) EvalResult {

	util.Log.Debugf("Eval ingress policy %s %s from pod %s on port %s %d", netpol.Namespace, netpol.Name, source.GetName(), toPort.Name, toPort.Num)

	if !util.Contains(netpol.Spec.PolicyTypes, nwv1.PolicyTypeIngress) {
		// netpol does not describe ingress
		util.Log.Tracef("Policy does not describe ingress %s %s", netpol.Namespace, netpol.Name)
		return NoMatch
	}

	if !dest.MatchPodSelector(netpol.Spec.PodSelector) {
		// netpol does not match source pod
		util.Log.Tracef("Policy does not match pod %+v %s", netpol.Spec.PodSelector, dest.GetName())
		return NoMatch
	}

	// does an ingress rule match the toPod and toPort?
	for _, eRule := range netpol.Spec.Ingress {
		if evalRule(netpol.Namespace, eRule.From, eRule.Ports, source, toPort) {
			return Allow
		}
	}

	util.Log.Debugf("Ingress denied for lack of a matching rule")
	return Deny
}

func evalEgress(source ConnectionSide, netpol nwv1.NetworkPolicy, dest ConnectionSide, toPort DestinationPort) EvalResult {
	util.Log.Debugf("Eval egress for policy %s %s to pod %s", netpol.Namespace, netpol.Name, dest.GetName())

	if !util.Contains(netpol.Spec.PolicyTypes, nwv1.PolicyTypeEgress) {
		util.Log.Tracef("Policy does not describe egress %s %s", netpol.Namespace, netpol.Name)
		// netpol does not describe egress
		return NoMatch
	}

	if !source.MatchPodSelector(netpol.Spec.PodSelector) {
		// netpol does not match source pod
		util.Log.Tracef("Policy does not match pod %+v %s", netpol.Spec.PodSelector, source.GetName())
		return NoMatch
	}

	// does an egress rule match the toPod and toPort?
	for _, eRule := range netpol.Spec.Egress {
		if evalRule(netpol.Namespace, eRule.To, eRule.Ports, dest, toPort) {
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
	other ConnectionSide,
	toPort DestinationPort,
) bool {

	// If any peers match otherPod, compare ports. If both match return true.
	for _, peer := range peers {
		var peerMatch bool
		if peer.IPBlock != nil {
			// "If this field [peer.IPBlock] is set then neither of the other fields can be."
			ipBlockMatch, err := other.MatchIPBlock(*peer.IPBlock)
			if err != nil {
				// Bail because by the time a user knows the podName this shouldn't be possible.
				util.Log.Panicf("error evaluating policy in namespace %s against pod %s", policyNamespace, other.GetName())
			}
			util.Log.Tracef("IPBlock compared %t %v %sv", ipBlockMatch, *peer.IPBlock, other.GetName())

			peerMatch = ipBlockMatch
		} else {
			// In the absence of peer.IPBlock, NamespaceSelector and PodSelector have meaning when they are nil.

			var namespaceMatch bool
			if peer.NamespaceSelector == nil {
				// "Otherwise it selects the Pods matching PodSelector in the policy's own Namespace"
				namespaceMatch = other.IsInNamespace(policyNamespace)
			} else {
				namespaceMatch = other.MatchNamespaceSelector(*peer.NamespaceSelector)
			}

			var podMatch bool
			if peer.PodSelector == nil {
				// "if present but empty, it selects all pods"
				podMatch = other.IsInCluster() // match all pods, but not external hosts
			} else {
				podMatch = other.MatchPodSelector(*peer.PodSelector)
			}

			util.Log.Tracef("Comparing peer selectors %v %v to pod labels %s", peer.NamespaceSelector, peer.PodSelector, other.GetName())
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

			if len(ports) == 0 {
				util.Log.Debugf("Peer match and empty port list so all ports allowed.")
				return true
			}
			util.Log.Debugf("Peer match, but port not found in policy %+v", toPort)
			return false
		}
	}

	util.Log.Tracef("evalRule did not match peers %+v on %s", peers, other.GetName())
	return false
}
