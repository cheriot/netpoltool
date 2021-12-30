package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	eval "github.com/cheriot/netpoltool/internal/app/netpoleval"
	"github.com/cheriot/netpoltool/internal/util"
)

func RenderCheckAccess(w io.Writer, portResults []eval.PortResult, dest *eval.ConnectionSide) error {
	if len(portResults) == 0 {
		fmt.Printf("No ports found on %s %s.\n", dest.Namespace.Name, dest.Pod.Name)
	}

	// api 3000 Allow
	//     Egress from ns-npt-0 pod-name-asdf Allow
	//     Ingress to ns-npt-1 pod-name-fdas Allow

	for _, portResult := range portResults {
		fmt.Fprintf(
			w,
			"%s %s %d %s\n",
			renderAllowSymbol(portResult.Allowed),
			portResult.ToPort.Name,
			portResult.ToPort.ContainerPort,
			renderAllow(portResult.Allowed))
		renderNetpolResults(w, "    Egress", portResult.Egress)
		renderNetpolResults(w, "    Ingress", portResult.Ingress)
	}

	return nil
}

func renderNetpolResults(w io.Writer, prefix string, nprs []eval.NetpolResult) {
	matching := util.Filter(nprs, func(npr eval.NetpolResult) bool { return npr.EvalResult != eval.NoMatch })
	if len(matching) == 0 {
		fmt.Fprintf(w, "%s: Allow (no matching policies)\n", prefix)
	}
	for _, npr := range matching {
		if npr.EvalResult != eval.NoMatch {
			fmt.Fprintf(w, "%s: %s %s %s\n", prefix, eval.EvalResultString(npr.EvalResult), npr.Netpol.Namespace, npr.Netpol.Name)
		}
	}
}

func renderAllowSymbol(isAllowed bool) string {
	if isAllowed {
		return "✓"
	}
	return "✗"
}
func renderAllow(isAllowed bool) string {
	if isAllowed {
		return "Allow"
	}
	return "Deny"
}

func RenderNetPolMatch(matches []nwv1.NetworkPolicy) {
	// symbol name policytypes ports ports
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintf(writer, "MATCH\tNAME\tPOLICY\tI-PORT\tE-PORT\n")
	defer writer.Flush()
	for _, np := range matches {
		fmt.Fprintf(
			writer,
			"✅\t%s\t%s\t%s\t%s\n",
			np.ObjectMeta.Name,
			renderPolicyTypes(np.Spec.PolicyTypes),
			renderIngressPorts(np.Spec.Ingress, np.Spec.PolicyTypes),
			renderEgressPorts(np.Spec.Egress, np.Spec.PolicyTypes),
		)
	}
}

func renderEgressPorts(rules []nwv1.NetworkPolicyEgressRule, pts []nwv1.PolicyType) string {
	if util.Contains(pts, nwv1.PolicyTypeEgress) && len(rules) == 0 {
		return "deny"
	}
	ports := make([]string, 0)
	for _, r := range rules {
		ports = append(ports, renderPorts(r.Ports))
	}
	return strings.Join(ports, ",")
}

func renderIngressPorts(rules []nwv1.NetworkPolicyIngressRule, pts []nwv1.PolicyType) string {
	if util.Contains(pts, nwv1.PolicyTypeIngress) && len(rules) == 0 {
		return "deny"
	}
	ports := make([]string, 0)
	for _, r := range rules {
		ports = append(ports, renderPorts(r.Ports))
	}
	return strings.Join(ports, ",")
}

func renderPolicyTypes(pts []nwv1.PolicyType) string {
	if len(pts) == 1 {
		return string(pts[0])
	} else if len(pts) == 2 {
		return fmt.Sprintf("%s,%s", string(pts[0]), string(pts[1]))
	}
	util.Log.Warnf("Unexpected NetworkPolicy#Spec#PolicyTypes %+v", pts)
	return ""
}

func renderPorts(npps []nwv1.NetworkPolicyPort) string {
	strs := make([]string, 0, len(npps))
	for _, npp := range npps {
		var port string
		if npp.Port == nil {
			port = "ALL"
		} else if npp.EndPort != nil {
			port = fmt.Sprintf("%s-%d", renderIntOrStr(npp.Port), npp.EndPort)
		} else {
			port = strings.ToLower(renderIntOrStr(npp.Port))
		}
		//fmt.Printf("renderPort %+v %s %s\n", npp, port, npp.Port.IntVal)
		strs = append(strs, port)
	}
	return strings.Join(strs, ",")
}

func renderIntOrStr(ios *intstr.IntOrString) string {
	if ios.Type == intstr.Int {
		return fmt.Sprintf("%d", ios.IntVal)
	}
	return ios.StrVal
}
