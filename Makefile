IMAGE_NAME ?= kubeagent
IMAGE_TAG  ?= latest
NAMESPACE  ?= kubeagent

.PHONY: build test docker-build deploy undeploy exec

# Build the Go binary locally
build:
	cd KubeAgent && CGO_ENABLED=0 go build -o ../bin/kubeagent .

# Run tests
test:
	cd KubeAgent && go test ./pkg/agent/... -v

# Build Docker image
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Deploy to K8s (create namespace, RBAC, secret, deployment)
deploy:
	kubectl apply -f deploy/namespace.yaml
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/secret.yaml
	kubectl apply -f deploy/deployment.yaml
	@echo ""
	@echo "Deployed! Wait for pod to be ready, then run:"
	@echo "  make exec MODE=analyze"

# Remove from K8s
undeploy:
	kubectl delete -f deploy/deployment.yaml --ignore-not-found
	kubectl delete -f deploy/rbac.yaml --ignore-not-found
	kubectl delete -f deploy/secret.yaml --ignore-not-found
	kubectl delete -f deploy/namespace.yaml --ignore-not-found

# Interactive exec into the pod (MODE = analyze | chat | kubecheck)
MODE ?= analyze
exec:
	kubectl exec -it -n $(NAMESPACE) deploy/kubeagent -- kubeagent $(MODE)
