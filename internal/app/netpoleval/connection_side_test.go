package netpoleval

import (
	"testing"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
)

func TestConnectionSide(t *testing.T) {

	cs := ConnectionSide{
		Pod: &corev1.Pod{
			Spec: corev1.PodSpec {
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
		},
	}
	Convey("GetPort", t, func() {
		Convey("Finds a port by number", func() {
			p, err := cs.GetPort("2000")
			So(err, ShouldBeNil)
			So(p, ShouldResemble, &corev1.ContainerPort{ContainerPort: 2000})
		})

		Convey("Finds a port by name", func() {
			p, err := cs.GetPort("foo")
			So(err, ShouldBeNil)
			So(p, ShouldResemble, &corev1.ContainerPort{Name: "foo", ContainerPort: 1000})
		})

		Convey("Fails for a name that doesn't exist", func() {
			_, err := cs.GetPort("doesnotexist")
			So(err, ShouldBeError)
		})

		Convey("Fails for a number that doesn't exist", func() {
			_, err := cs.GetPort("3000")
			So(err, ShouldBeError)
		})
	})
}