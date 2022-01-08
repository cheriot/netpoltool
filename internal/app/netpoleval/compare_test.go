package netpoleval

import (
	"net"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestPortContains(t *testing.T) {
	containerPort := DestinationPort{
		Name:     "healthCheck",
		Num:      3000,
		Protocol: corev1.ProtocolTCP,
	}

	matchingProtocol := containerPort.Protocol
	matchingNum := intstr.FromInt(int(containerPort.Num))
	matchingName := intstr.FromString(containerPort.Name)

	differentProtocol := corev1.ProtocolUDP
	differentNum := intstr.FromInt(int(containerPort.Num + 1))
	differentName := intstr.FromString(containerPort.Name + "Different")

	testPort := func(policyProtocol corev1.Protocol, policyPort intstr.IntOrString, assert convey.Assertion) {
		isMatch := PortContains(
			nwv1.NetworkPolicyPort{
				Protocol: &policyProtocol,
				Port:     &policyPort,
			},
			containerPort,
		)
		So(isMatch, assert)
	}

	Convey("Different protocol, matching port", t, func() {
		testPort(differentProtocol, matchingNum, ShouldBeFalse)
	})

	Convey("Matching protocol, matching port int", t, func() {
		testPort(matchingProtocol, matchingNum, ShouldBeTrue)
	})

	Convey("Matching protocol, different port int", t, func() {
		testPort(matchingProtocol, differentNum, ShouldBeFalse)
	})

	Convey("Matching protocol, matching port name", t, func() {
		testPort(matchingProtocol, matchingName, ShouldBeTrue)
	})

	Convey("Matching protocol, different port name", t, func() {
		testPort(matchingProtocol, differentName, ShouldBeFalse)
	})

	testRange := func(low, high int32, assert convey.Assertion) {
		lowWrap := intstr.FromInt(int(low))
		isMatch := PortContains(
			nwv1.NetworkPolicyPort{
				Protocol: &matchingProtocol,
				Port:     &lowWrap,
				EndPort:  &high,
			},
			containerPort,
		)
		So(isMatch, assert)
	}

	Convey("Matching protocol, within port range", t, func() {
		testRange(
			containerPort.Num-1,
			containerPort.Num+1,
			ShouldBeTrue,
		)
	})

	Convey("Matching protocol, lower bound port range", t, func() {
		testRange(
			containerPort.Num,
			containerPort.Num+1,
			ShouldBeTrue,
		)
	})

	Convey("Matching protocol, upper bound port range", t, func() {
		testRange(
			containerPort.Num-1,
			containerPort.Num,
			ShouldBeTrue,
		)
	})

	Convey("Matching protocol, different port range", t, func() {
		testRange(
			containerPort.Num+1,
			containerPort.Num+2,
			ShouldBeFalse,
		)
	})

	Convey("Missing port means Allow all ports", t, func() {
		isMatch := PortContains(
			nwv1.NetworkPolicyPort{
				Protocol: &containerPort.Protocol,
			},
			containerPort,
		)
		So(isMatch, ShouldBeTrue)
	})
}

func TestMatchLabelSelectors(t *testing.T) {
	podLabels := map[string]string{
		"app":       "fancything",
		"component": "graphql",
		"zone":      "web",
	}

	Convey("MatchLabels", t, func() {
		matchWithLabelSelector := func(selectLabels map[string]string) bool {
			return MatchLabelSelector(
				metav1.LabelSelector{
					MatchLabels: selectLabels,
				},
				podLabels,
			)
		}

		Convey("All selectors match - pod has additional labels", func() {
			So(
				matchWithLabelSelector(
					map[string]string{
						"zone": "web",
						"app":  "fancything",
					},
				),
				ShouldBeTrue)
		})

		Convey("Some selectors match - others have a different value", func() {
			So(
				matchWithLabelSelector(
					map[string]string{
						"zone": "web",
						"app":  "oldthing",
					},
				),
				ShouldBeFalse)
		})

		Convey("Some selectors match - others are missing", func() {
			So(
				matchWithLabelSelector(
					map[string]string{
						"zone": "web",
						"app":  "fancything",
						"foo":  "bar",
					},
				),
				ShouldBeFalse)
		})
	})

	Convey("MatchExpressions", t, func() {
		validExpressions := func() metav1.LabelSelector {
			return metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "zone", Operator: metav1.LabelSelectorOpIn, Values: []string{"assets", "web"}},
					{Key: "zone", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"admin"}},
					{Key: "app", Operator: metav1.LabelSelectorOpExists},
					{Key: "foo", Operator: metav1.LabelSelectorOpDoesNotExist},
				},
			}
		}

		withExpression := func(key string, op metav1.LabelSelectorOperator, values []string) metav1.LabelSelector {
			return metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      key,
					Operator: op,
					Values:   values,
				}},
			}
		}

		Convey("Matching expression with each operator.", func() {
			So(MatchLabelSelector(validExpressions(), podLabels), ShouldBeTrue)
		})

		Convey("Unmatch with In.", func() {
			So(MatchLabelSelector(withExpression("zone", metav1.LabelSelectorOpIn, []string{"admin"}), podLabels), ShouldBeFalse)
		})

		Convey("Unmatch with NotIn.", func() {
			So(MatchLabelSelector(withExpression("zone", metav1.LabelSelectorOpNotIn, []string{"web"}), podLabels), ShouldBeFalse)
		})

		Convey("Unmatch with Exists.", func() {
			So(MatchLabelSelector(withExpression("foo", metav1.LabelSelectorOpExists, []string{}), podLabels), ShouldBeFalse)
		})

		Convey("Unmatch with DoesNotExists.", func() {
			So(MatchLabelSelector(withExpression("zone", metav1.LabelSelectorOpDoesNotExist, []string{}), podLabels), ShouldBeFalse)
		})
	})

	Convey("Empty selectors match", t, func() {
		So(MatchLabelSelector(metav1.LabelSelector{}, podLabels), ShouldBeTrue)
	})

	Convey("Valid MatchLabels with invalid MatchExpressions.", t, func() {
		labelSelector := metav1.LabelSelector{
			MatchLabels: podLabels,
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "zone",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}},
		}
		So(MatchLabelSelector(labelSelector, podLabels), ShouldBeFalse)

		// Double check that it's the MatchExpression failing
		labelSelector.MatchExpressions = []metav1.LabelSelectorRequirement{}
		So(MatchLabelSelector(labelSelector, podLabels), ShouldBeTrue)
	})

	// Invalid MatchLabels with valid MatchExpressions
	Convey("Invalid MatchLabels with valid MatchExpressions.", t, func() {
		labelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{"foo": "bar"},
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "zone",
				Operator: metav1.LabelSelectorOpExists,
			}},
		}
		So(MatchLabelSelector(labelSelector, podLabels), ShouldBeFalse)

		// Double check that it's the MatchLabels failing
		labelSelector.MatchLabels = map[string]string{}
		So(MatchLabelSelector(labelSelector, podLabels), ShouldBeTrue)
	})
}

func TestMatchIPBlock(t *testing.T) {
	Convey("TestMatchIPBlock", t, func() {
		ipBlock := nwv1.IPBlock{
			CIDR: "10.1.1.0/16",
		}
		inCidrStr := "10.1.1.0"
		inCidrIP := net.ParseIP(inCidrStr)
		outsideCidrStr := "20.1.1.0"
		outsideCidrIp := net.ParseIP(outsideCidrStr)

		Convey("Matches an IP in the ipBlock", func() {
			isMatch, err := MatchIPBlock(ipBlock, inCidrIP, inCidrStr)
			So(err, ShouldBeNil)
			So(isMatch, ShouldBeTrue)
		})

		Convey("Does not match an IP outside the ipBlock", func() {
			isMatch, err := MatchIPBlock(ipBlock, outsideCidrIp, outsideCidrStr)
			So(err, ShouldBeNil)
			So(isMatch, ShouldBeFalse)
		})

		Convey("Does not match an IP inside the ipBlock that's in the Except list", func() {
			ipBlockExcept := ipBlock.DeepCopy()
			ipBlockExcept.Except = []string{inCidrStr}
			isMatch, err := MatchIPBlock(*ipBlockExcept, inCidrIP, inCidrStr)
			So(err, ShouldBeNil)
			So(isMatch, ShouldBeFalse)
		})
	})
}
