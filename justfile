# Project version
version := "0.0.1"

# Image settings
image_tag_base := "warframe.market/warframe-market-operator"
img             := env_var_or_default("IMG", "controller:latest")
bundle_img      := image_tag_base + "-bundle:v" + version
catalog_img     := image_tag_base + "-catalog:v" + version

# Container tool (docker or podman)
container_tool := env_var_or_default("CONTAINER_TOOL", "docker")

# Kubernetes version for envtest binaries
envtest_k8s_version := "1.33"

# Local tool binaries (absolute paths so cd into subdirs works)
controller_gen := justfile_directory() + "/bin/controller-gen"
kustomize      := justfile_directory() + "/bin/kustomize"
envtest        := justfile_directory() + "/bin/setup-envtest"
golangci_lint  := justfile_directory() + "/bin/golangci-lint"

# Tool versions
controller_gen_version := "v0.18.0"
kustomize_version      := "v5.6.0"
golangci_lint_version  := "v2.1.0"

# Default recipe
default: build

# Install all required Go tools into ./bin/
tools:
    mkdir -p bin
    GOBIN=$(pwd)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@{{controller_gen_version}}
    GOBIN=$(pwd)/bin go install sigs.k8s.io/kustomize/kustomize/v5@{{kustomize_version}}
    GOBIN=$(pwd)/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
    GOBIN=$(pwd)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{golangci_lint_version}}

# --- Development ---

# Generate WebhookConfiguration, ClusterRole and CRD manifests
manifests:
    {{controller_gen}} rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate DeepCopy method implementations
generate:
    {{controller_gen}} object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Run go fmt
fmt:
    go fmt ./...

# Run go vet
vet:
    go vet ./...

# Run unit and integration tests
test: manifests generate fmt vet
    KUBEBUILDER_ASSETS="$({{envtest}} use {{envtest_k8s_version}} -p path)" go test $(go list ./... | grep -v /e2e) -coverprofile cover.out

# Run e2e tests via k3s Testcontainer (Docker required)
test-e2e: manifests generate fmt vet
    go test ./test/e2e/ -v -ginkgo.v -timeout 10m

# Run golangci-lint
lint:
    {{golangci_lint}} run

# Run golangci-lint and auto-fix
lint-fix:
    {{golangci_lint}} run --fix

# Verify golangci-lint configuration
lint-config:
    {{golangci_lint}} config verify

# --- Build ---

# Build manager binary
build: manifests generate fmt vet
    go build -o bin/manager cmd/main.go

# Build wfmctl CLI binary
cli-build: fmt vet
    go build -o bin/wfmctl ./cmd/wfmctl

# Install wfmctl CLI to GOPATH/bin
cli-install: fmt vet
    go install ./cmd/wfmctl

cli: cli-build cli-install

# Run controller from host
run: manifests generate fmt vet
    go run ./cmd/main.go

# Build docker image
docker-build:
    {{container_tool}} build -t {{img}} .

# Push docker image
docker-push:
    {{container_tool}} push {{img}}

# Build and push multi-platform docker image
docker-buildx platforms="linux/arm64,linux/amd64":
    sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
    -{{container_tool}} buildx create --name warframe-market-operator-builder
    {{container_tool}} buildx use warframe-market-operator-builder
    -{{container_tool}} buildx build --push --platform={{platforms}} --tag {{img}} -f Dockerfile.cross .
    -{{container_tool}} buildx rm warframe-market-operator-builder
    rm Dockerfile.cross

# Private helper: sets the controller image in config/manager (runs from that dir)
[working-directory: 'config/manager']
_set-image img=img:
    {{kustomize}} edit set image controller={{img}}

# Generate consolidated install.yaml in dist/
build-installer: manifests generate
    mkdir -p dist
    just _set-image {{img}}
    {{kustomize}} build config/default > dist/install.yaml

# --- Deployment ---

# Install CRDs into the cluster
install: manifests
    {{kustomize}} build config/crd | kubectl apply -f -

# Uninstall CRDs from the cluster
uninstall: manifests
    {{kustomize}} build config/crd | kubectl delete --ignore-not-found=true -f -

# Deploy controller to the cluster
deploy: manifests
    just _set-image {{img}}
    {{kustomize}} build config/default | kubectl apply -f -

# Undeploy controller from the cluster
undeploy:
    {{kustomize}} build config/default | kubectl delete --ignore-not-found=true -f -

# --- Bundle (OLM) ---

# Generate OLM bundle manifests
bundle: manifests
    operator-sdk generate kustomize manifests -q
    just _set-image {{img}}
    {{kustomize}} build config/manifests | operator-sdk generate bundle -q --overwrite --version {{version}}
    operator-sdk bundle validate ./bundle

# Build bundle image
bundle-build:
    {{container_tool}} build -f bundle.Dockerfile -t {{bundle_img}} .

# Push bundle image
bundle-push:
    {{container_tool}} push {{bundle_img}}

# Build catalog image
catalog-build:
    opm index add --container-tool {{container_tool}} --mode semver --tag {{catalog_img}} --bundles {{bundle_img}}

# Push catalog image
catalog-push:
    {{container_tool}} push {{catalog_img}}
