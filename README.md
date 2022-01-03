# netpoltool

CLI evaluation of Kubernetes NetworkPolicys with detailed output helpful for debugging. Given source and destination pods, identify the NetworkPolicies that apply and whether a connection is allowed.

### Requirements
* golang 1.18beta

### Install
```
go1.18beta1 install github.com/cheriot/netpoltool/cmd/netpoltool@latest
```

### Run
netpoltool eval --namespace=_sourceNamespace_ --pod=_sourcePod_ --to-namespace=_destinationNamespace_ --to-pod=_destinationPod_

```
Usage:
  main [OPTIONS] eval [eval-OPTIONS]

Given source and destination pods, evaluate if Network Policies allow the source pod to access any ports on the destination pod.

Application Options:
      --log-level=        Log level (trace, debug, info, warning, error, fatal, panic).
  -v, --verbose           Show verbose debug information
      --kubeconfig=       Absolute path to the kubeconfig file. Default to ~/.kube/config.
      --namespace=        Namespace of the pod creating the connection.

Help Options:
  -h, --help              Show this help message

[eval command options]
      -p, --pod=
          --to-namespace= Namespace of the pod receiving the connection.
          --to-pod=       Name of the pod receiving the connection.
          --to-port=      Number or name of the port to connect to.
```
