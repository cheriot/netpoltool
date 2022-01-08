package netpoleval

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cheriot/netpoltool/internal/util"
)

type ConnectionSide interface {
	GetName() string
	MatchNamespaceSelector(metav1.LabelSelector) bool
	MatchPodSelector(metav1.LabelSelector) bool
	MatchIPBlock(nwv1.IPBlock) (bool, error)
	IsInNamespace(string) bool
	IsOnNode(string) bool
	IsInCluster() bool
	GetPolicies() []nwv1.NetworkPolicy
	GetPorts() []DestinationPort
}

type DestinationPort struct {
	IsInCluster bool // Does this port represent something in the *current* k8s cluster?
	Name        string
	Num         int32
	Protocol    corev1.Protocol
}

func NewPodConnection(pod *corev1.Pod, ns *corev1.Namespace, policies []nwv1.NetworkPolicy, portNameOrNum string) (*PodConnection, error) {
	ipStr := pod.Status.PodIP // Do we need to check the ipv6 addr in Status.PodIPs?
	if ipStr == "" {
		// A new pod may not have been assigned an IP yet. An expected case, but inform the user that it's different
		// than a NetworkPolicy evaluating to false.
		return nil, fmt.Errorf("blank IP so ipBlock netpols cannot be evaluated. %s %s phase: %s", pod.Namespace, pod.Name, pod.Status.Phase)
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP \"%s\" on pod %s/%s phase: %s", ipStr, pod.Namespace, pod.Name, pod.Status.Phase)
	}

	var ports []DestinationPort
	if portNameOrNum != "" {
		p, err := portFromIdentifier(pod, portNameOrNum)
		if err != nil {
			return nil, fmt.Errorf("invalid destination: %w", err)
		}
		ports = []DestinationPort{p}
	} else {
		ports = podPorts(pod)
	}

	util.Log.Debugf("New PodConnection %s %s %s %+v", pod.Namespace, pod.Name, ip, pod.Labels)
	return &PodConnection{
		ports:     ports,
		ip:        ip,
		Pod:       pod,
		Namespace: ns,
		Policies:  policies,
	}, nil
}

func NewExternalConnection(ip string, port string, protocol string) (ConnectionSide, error) {
	num, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("invalid port number %s: %w", port, err)
	}

	IP := net.ParseIP(ip)
	if IP == nil {
		return nil, fmt.Errorf("invalid IP %s", ip)
	}

	return &ExternalConnection{
		ipStr: ip,
		IP:    IP,
		Port: DestinationPort{
			IsInCluster: false,
			Name:        "",
			Num:         int32(num),
			Protocol:    protocolFromString(protocol),
		},
	}, nil
}

type ExternalConnection struct {
	ipStr string
	IP    net.IP
	Port  DestinationPort
}

func (c *ExternalConnection) GetName() string {
	return fmt.Sprintf("%s:%d", c.ipStr, c.Port.Num)
}

func (c *ExternalConnection) MatchNamespaceSelector(metav1.LabelSelector) bool {
	return false
}

func (c *ExternalConnection) MatchPodSelector(metav1.LabelSelector) bool {
	return false
}

func (c *ExternalConnection) MatchIPBlock(ipBlock nwv1.IPBlock) (bool, error) {
	return MatchIPBlock(ipBlock, c.IP, c.ipStr)
}

func (c *ExternalConnection) IsInNamespace(string) bool {
	return false
}

func (c *ExternalConnection) IsOnNode(name string) bool {
	return false
}

func (c *ExternalConnection) GetPolicies() []nwv1.NetworkPolicy {
	return nil
}

func (c *ExternalConnection) IsInCluster() bool {
	return false
}

func (c *ExternalConnection) GetPorts() []DestinationPort {
	return []DestinationPort{c.Port}
}

type PodConnection struct {
	ports     []DestinationPort
	ip        net.IP
	Pod       *corev1.Pod
	Namespace *corev1.Namespace
	Policies  []nwv1.NetworkPolicy
}

func (c *PodConnection) GetName() string {
	return c.Namespace.Name + "/" + c.Pod.Name
}

func (c *PodConnection) MatchNamespaceSelector(labelSelector metav1.LabelSelector) bool {
	return MatchLabelSelector(labelSelector, c.Namespace.Labels)
}

func (c *PodConnection) MatchPodSelector(labelSelector metav1.LabelSelector) bool {
	return MatchLabelSelector(labelSelector, c.Pod.Labels)
}

func (c *PodConnection) IsInNamespace(n string) bool {
	return c.Namespace.Name == n
}

func (c *PodConnection) IsOnNode(name string) bool {
	return c.Pod.Spec.NodeName == name
}

func (c *PodConnection) GetPolicies() []nwv1.NetworkPolicy {
	return c.Policies
}

func (c *PodConnection) IsInCluster() bool {
	return true
}

func (c *PodConnection) MatchIPBlock(ipBlock nwv1.IPBlock) (bool, error) {
	return MatchIPBlock(ipBlock, c.ip, c.Pod.Status.PodIP)
}

func (c *PodConnection) GetPorts() []DestinationPort {
	return c.ports
}

func podPorts(pod *corev1.Pod) []DestinationPort {
	ports := make([]DestinationPort, 0)
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			ports = append(ports, DestinationPort{
				IsInCluster: true,
				Name:        p.Name,
				Num:         p.ContainerPort,
				Protocol:    p.Protocol,
			})
		}
	}
	return ports
}

func portFromIdentifier(pod *corev1.Pod, nameOrNum string) (DestinationPort, error) {
	num, err := strconv.Atoi(nameOrNum)
	isNum := err == nil

	for _, p := range podPorts(pod) {
		if p.Name == nameOrNum || isNum && p.Num == int32(num) {
			return p, nil
		}
	}
	return DestinationPort{}, fmt.Errorf("unable to find port %s on pod %s %s", nameOrNum, pod.Namespace, pod.Name)
}

func protocolFromString(protocol string) corev1.Protocol {
	switch strings.ToUpper(protocol) {
	case "UDP":
		return corev1.ProtocolUDP
	case "SCTP":
		return corev1.ProtocolSCTP
	}
	return corev1.ProtocolTCP
}
