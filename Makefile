test:
	go test -v ./...

fmt:
	go mod tidy
	gofmt -w .
	goimports --local github.com/cheriot/netpoltool/ -w .

gen:
	rm -rf ./testdata/generated-yamls/*
	go run cmd/testresources/main.go ./testdata/generated-yamls

run:
	go run cmd/netpoltool/main.go --log-level=trace eval --namespace=front-end --pod=serve-pod-info --to-namespace=ns-npt-1 --to-pod=serve-pod-info -vv

run-ip:
	go run cmd/netpoltool/main.go --log-level=trace eval --namespace=ns-npt-0 --pod=serve-pod-info --to-ext-ip=127.0.0.1 --to-port=3000 -vv

build:
	go build -o netpoltool cmd/netpoltool/main.go

clitools-run:
	docker run --rm -p "3000:3000" -e "PUBLIC_POD_NAME=foo-pod-asdf" --name mockpod clitools:latest

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
	kubectl apply -Rf ./testdata/generated-yamls/

int-delete:
	kubectl delete -Rf ./testdata/generated-yamls/
