package netpoleval

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConnectionSide(t *testing.T) {
	ns := &corev1.Namespace{}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "PodOne",
			Namespace: ns.Name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Ports: []corev1.ContainerPort{
						{Name: "foo", ContainerPort: 1000},
					},
				},
				{
					Ports: []corev1.ContainerPort{
						{ContainerPort: 2000},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			PodIP: "127.0.0.1",
		},
	}

	Convey("NewPodConnection", t, func() {
		Convey("Finds a port by number", func() {
			p, err := NewPodConnection(pod, ns, []nwv1.NetworkPolicy{}, "2000")
			So(err, ShouldBeNil)
			So(p.GetPorts()[0], ShouldResemble, DestinationPort{IsInCluster: true, Name: "", Num: 2000})
		})

		Convey("Finds a port by name", func() {
			p, err := NewPodConnection(pod, ns, []nwv1.NetworkPolicy{}, "foo")
			So(err, ShouldBeNil)
			So(p.GetPorts()[0], ShouldResemble, DestinationPort{IsInCluster: true, Name: "foo", Num: 1000})
		})

		Convey("Fails for a name that doesn't exist", func() {
			_, err := NewPodConnection(pod, ns, []nwv1.NetworkPolicy{}, "doesnotexist")
			So(err, ShouldBeError)
		})

		Convey("Fails for a number that doesn't exist", func() {
			_, err := NewPodConnection(pod, ns, []nwv1.NetworkPolicy{}, "12345")
			So(err, ShouldBeError)
		})
	})
}
