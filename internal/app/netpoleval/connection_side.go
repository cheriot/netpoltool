package netpoleval

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
)

type ConnectionSide struct {
	Pod       *corev1.Pod
	Namespace *corev1.Namespace
	Policies  []nwv1.NetworkPolicy
}

func (c *ConnectionSide) GetContainerPorts() []corev1.ContainerPort {
	ports := make([]corev1.ContainerPort, 0)
	for _, c := range c.Pod.Spec.Containers {
		ports = append(ports, c.Ports...)
	}
	return ports
}

func (c *ConnectionSide) GetPort(nameOrNum string) (*corev1.ContainerPort, error) {
	var isNum bool
	num, err := strconv.Atoi(nameOrNum)
	isNum = err == nil

	for _, p := range c.GetContainerPorts() {
		if p.Name == nameOrNum || isNum && p.ContainerPort == int32(num) {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("unable to find port %s on pod %s %s", nameOrNum, c.Pod.Namespace, c.Pod.Name)
}