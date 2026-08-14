# warframe-market-operator

A Kubernetes operator that watches Warframe Market prices and sends notifications when items or riven mods drop below a configured threshold.

## Custom Resources

- **ItemPriceWatch** — monitors a specific item (by slug) and notifies when the cheapest sell order falls below a threshold
- **RivenPriceWatch** — monitors riven auctions for a weapon with stat/quality filters and notifies on matching auctions below threshold

## Prerequisites

Install system tools (once):

```sh
brew install go docker just kubectl
```

Install project-local Go tools into `./bin/` (once per checkout):

```sh
just tools
```

For e2e tests, Docker is required (k3s starts automatically via Testcontainers — no external cluster needed).

## Development

```sh
# Generate manifests and code
just generate
just manifests

# Run unit + integration tests
just test

# Run e2e tests (starts k3s in Docker automatically)
just test-e2e

# Lint
just lint
```

## Deploy to Cluster

```sh
# Build and push the operator image
just docker-build docker-push IMG=<registry>/warframe-market-operator:tag

# Install CRDs
just install

# Deploy the operator
just deploy IMG=<registry>/warframe-market-operator:tag

# Apply sample CRs
kubectl apply -k config/samples/
```

## Uninstall

```sh
kubectl delete -k config/samples/
just undeploy
just uninstall
```

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

