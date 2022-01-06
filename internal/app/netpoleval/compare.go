package netpoleval

import (
	"fmt"
	"net"

	//"github.com/cheriot/netpoltool/internal/util"
	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/strings/slices"
)

func PortContains(rulePort nwv1.NetworkPolicyPort, toPort corev1.ContainerPort) bool {
	if *rulePort.Protocol != toPort.Protocol {
		// if nil, default to TCP
		return false
	}

	if rulePort.Port == nil {
		// no ports means all ports
		return true
	}

	if rulePort.Port.Type == intstr.String {
		// A port name means there can't be a range.
		return rulePort.Port.StrVal == toPort.Name
	}

	if rulePort.EndPort != nil {
		// Check range [Port, EndPort]
		return rulePort.Port.IntVal <= toPort.ContainerPort &&
			toPort.ContainerPort <= *rulePort.EndPort
	}

	return rulePort.Port.IntVal == toPort.ContainerPort
}

func MatchLabelSelector(podSelector metav1.LabelSelector, podLabels map[string]string) bool {
	if len(podSelector.MatchLabels)+len(podSelector.MatchExpressions) == 0 {
		// empty selectors match all pods
		return true
	}

	// Because all pod selectors are AND'ed we can bail as
	// soon as we find one that doesn't match.
	for k, v := range podSelector.MatchLabels {
		if podVal, ok := podLabels[k]; !ok || podVal != v {
			return false
		}
	}

	for _, lrs := range podSelector.MatchExpressions {
		podVal, ok := podLabels[lrs.Key]
		switch lrs.Operator {
		case metav1.LabelSelectorOpIn:
			if !ok || !slices.Contains(lrs.Values, podVal) {
				return false
			}
		case metav1.LabelSelectorOpNotIn:
			if !ok || slices.Contains(lrs.Values, podVal) {
				return false
			}
		case metav1.LabelSelectorOpExists:
			if !ok {
				return false
			}
		case metav1.LabelSelectorOpDoesNotExist:
			if ok {
				return false
			}
		}
	}
	return true
}

func MatchIPBlock(ipBlock nwv1.IPBlock, ip net.IP, ipStr string) (bool, error) {
	for _, exceptIpStr := range ipBlock.Except {
		if ipStr == exceptIpStr {
			return false, nil
		}
	}

	_, ipNet, err := net.ParseCIDR(ipBlock.CIDR)
	if err != nil {
		return false, fmt.Errorf("unable to parse ipBlock.CIDR %s", ipBlock.CIDR)
	}

	return ipNet.Contains(ip), nil
}

func MatchIPBlockToPod(ipBlock nwv1.IPBlock, pod *corev1.Pod) (bool, error) {
	ipStr := pod.Status.PodIP // Do we need to check the ipv6 addr in Status.PodIPs?
	if ipStr == "" {
		// A new pod may not have been assigned an IP yet. An expected case, but inform the user that it's different
		// than a NetworkPolicy evaluating to false.
		return false, fmt.Errorf("blank IP so ipBlock netpols cannot be evaluated.")
	}
	ip := net.ParseIP(ipStr)
	return MatchIPBlock(ipBlock, ip, ipStr)
}
