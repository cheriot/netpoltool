package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/fatih/color"

	eval "github.com/cheriot/netpoltool/internal/app/netpoleval"
	"github.com/cheriot/netpoltool/internal/util"
)

var (
	red   = color.New(color.FgRed).SprintfFunc()
	green = color.New(color.FgGreen).SprintfFunc()
)

type ConsoleView struct {
	Writer *bufio.Writer
	Verbosity
}

func NewConsoleView(v int) ConsoleView {
	return ConsoleView{
		Writer:    bufio.NewWriter(os.Stdout),
		Verbosity: GetVerbosity(v),
	}
}

func (c ConsoleView) Flush() {
	c.Writer.Flush()
}

type Verbosity uint8

const (
	Default           = iota // (not verbose) Show ports and their results
	DetailMatching           // (-v) Show all ports and the network policies that match both pods
	DetailNotMatching        // (-vv) Show all ports and all network policies
)

func GetVerbosity(flags int) Verbosity {
	if flags <= 0 {
		return Default
	} else if flags == 1 {
		return DetailMatching
	}
	return DetailNotMatching
}

func RenderCheckAccess(v ConsoleView, portResults []eval.PortResult, source, dest *eval.ConnectionSide) error {
	color.New(color.FgRed).SprintfFunc()
	if len(portResults) == 0 {
		fmt.Printf("No ports found on %s %s.\n", dest.Namespace.Name, dest.Pod.Name)
	}

	// api 3000 Allow
	//     Egress from ns-npt-0 pod-name-asdf Allow
	//     Ingress to ns-npt-1 pod-name-fdas Allow

	accessibleCount := 0
	for _, portResult := range portResults {
		if portResult.Allowed {
			accessibleCount++
		}

		fmt.Fprintf(
			v.Writer,
			"%s %s %d %s\n",
			renderAllowSymbol(portResult.Allowed),
			portResult.ToPort.Name,
			portResult.ToPort.ContainerPort,
			renderAllow(portResult.Allowed))

		if v.Verbosity > Default {
			fmt.Fprintf(v.Writer, "      %s Egress from pod %s/%s\n", renderAllowSymbol(portResult.EgressAllowed), source.Namespace.Name, dest.Pod.Name)
			renderNetpolResults(v, "            ", portResult.Egress)
			fmt.Fprintf(v.Writer, "      %s Ingress to pod %s/%s\n", renderAllowSymbol(portResult.IngressAllowed), dest.Namespace.Name, dest.Pod.Name)
			renderNetpolResults(v, "            ", portResult.Ingress)
		}
	}

	if accessibleCount == 0 {
		// print message and trigger a non-zero exit code
		return fmt.Errorf("no ports accessible")
	}
	return nil
}

func renderNetpolResults(v ConsoleView, prefix string, nprs []eval.NetpolResult) {
	matching := util.Filter(nprs, func(npr eval.NetpolResult) bool { return npr.EvalResult != eval.NoMatch })

	var viewable []eval.NetpolResult
	if v.Verbosity < DetailNotMatching {
		viewable = matching
	} else {
		viewable = nprs
	}

	if len(matching) == 0 {
		fmt.Fprintf(v.Writer, "%s%s (no matching policies)\n", prefix, renderEvalResult(eval.Allow))
	}
	for _, npr := range viewable {
		if npr.EvalResult != eval.NoMatch {
			fmt.Fprintf(v.Writer, "%s%s from NetworkPolicy %s/%s\n", prefix, renderEvalResult(npr.EvalResult), npr.Netpol.Namespace, npr.Netpol.Name)
		}
	}
}

func renderAllowSymbol(isAllowed bool) string {
	if isAllowed {
		return green("✓")
	}
	return red("✗")
}

func renderAllow(isAllowed bool) string {
	if isAllowed {
		return "Allow"
	}
	return "Deny"
}

func renderEvalResult(er eval.EvalResult) string {
	switch er {
	case eval.Allow:
		return green("Allow")
	case eval.Deny:
		return red("Deny")
	case eval.NoMatch:
		return "No Match"
	}
	return "Unknown"
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
