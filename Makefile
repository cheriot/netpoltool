fmt:
	go mod tidy
	gofmt -w .
	goimports --local github.com/cheriot/netpoltool/ -w .

run:
	go run cmd/netpoltool/main.go --log-level=trace eval --namespace=ns-npt-0 --pod=serve-pod-info --to-namespace=ns-npt-1 --to-pod=serve-pod-info -vv

build:
	go build -o netpoltool cmd/netpoltool/main.go

clitools-build:
	docker build testdata/images/clitools --tag clitools:latest

clitools-push:
	docker tag clitools:latest cheriot/clitools:latest
	docker push cheriot/clitools:latest

convey:
	$$(go env GOPATH)/bin/goconvey

int-cluster-create:
	kind create cluster --name test-cluster --wait 100s

int-cluster-delete:
	kind delete cluster --name test-cluster

int-apply:
	kubectl apply -f testdata/ns-npt-0
	kubectl apply -f testdata/ns-npt-1

int-delete:
	kubectl delete ns ns-npt-0
	kubectl delete ns ns-npt-1