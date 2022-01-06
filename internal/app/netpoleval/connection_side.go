package netpoleval

import (
	"fmt"
	"net"
	"strconv"

	"github.com/cheriot/netpoltool/internal/util"
	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConnectionSide interface {
	GetName() string
	MatchNamespaceSelector(metav1.LabelSelector) bool
	MatchPodSelector(metav1.LabelSelector) bool
	MatchIPBlock(nwv1.IPBlock) (bool, error)
	IsInNamespace(string) bool
	IsOnNode(string) bool
	IsPod() bool
	GetPolicies() []nwv1.NetworkPolicy
}

func NewPodConnection(pod *corev1.Pod, ns *corev1.Namespace, policies []nwv1.NetworkPolicy) (*PodConnection, error) {
	ipStr := pod.Status.PodIP // Do we need to check the ipv6 addr in Status.PodIPs?
	if ipStr == "" {
		// A new pod may not have been assigned an IP yet. An expected case, but inform the user that it's different
		// than a NetworkPolicy evaluating to false.
		return nil, fmt.Errorf("blank IP so ipBlock netpols cannot be evaluated.")
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP %s on pod %s/%s", ipStr, pod.Namespace, pod.Name)
	}

	util.Log.Debugf("New PodConnection %s %s %s %+v", pod.Namespace, pod.Name, ip, pod.Labels)
	return &PodConnection{
		ip:        ip,
		Pod:       pod,
		Namespace: ns,
		Policies:  policies,
	}, nil
}

func NewExternalConnection(ip string, port string) (ConnectionSide, error) {
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
		Port:  num,
	}, nil
}

type ExternalConnection struct {
	ipStr string
	IP    net.IP
	Port  int
}

func (c *ExternalConnection) GetName() string {
	return fmt.Sprintf("%s:%d", c.ipStr, c.Port)
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

func (c *ExternalConnection) IsPod() bool {
	return false
}

type PodConnection struct {
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

func (c *PodConnection) IsPod() bool {
	return true
}

func (c *PodConnection) MatchIPBlock(ipBlock nwv1.IPBlock) (bool, error) {
	return MatchIPBlock(ipBlock, c.ip, c.Pod.Status.PodIP)
}

func (c *PodConnection) GetContainerPorts() []corev1.ContainerPort {
	ports := make([]corev1.ContainerPort, 0)
	for _, c := range c.Pod.Spec.Containers {
		ports = append(ports, c.Ports...)
	}
	return ports
}

func (c *PodConnection) GetPort(nameOrNum string) (*corev1.ContainerPort, error) {
	num, err := strconv.Atoi(nameOrNum)
	isNum := err == nil

	for _, p := range c.GetContainerPorts() {
		if p.Name == nameOrNum || isNum && p.ContainerPort == int32(num) {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("unable to find port %s on pod %s %s", nameOrNum, c.Pod.Namespace, c.Pod.Name)
}
