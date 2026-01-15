# Team Operator

Kubernetes operator for managing Posit Team deployments.

## Project Structure

- **`api/`**: Kubernetes API/CRD definitions (core, product, keycloak, templates)
- **`cmd/`**: Main operator entry point
- **`internal/`**: Core operator logic and controllers
- **`config/`**: Kubernetes manifests and Kustomize configurations
- **`dist/chart/`**: Helm chart for deployment
- **`flightdeck/`**: Landing page dashboard component (separate Go module)
- **`client-go/`**: Generated Kubernetes client code
- **`docs/`**: User and contributor documentation

## Build and Development

```bash
just build          # Build operator binary to ./bin/team-operator
just test           # Run go tests
just run            # Run operator locally from source
just deps           # Install dependencies
just mgenerate      # Regenerate manifests after API changes
just helm-lint      # Lint Helm chart
just helm-template  # Render Helm templates locally
just helm-install   # Install operator via Helm
just helm-uninstall # Uninstall operator via Helm
```

## Namespaces

- **`posit-team-system`**: Where the operator runs
- **`posit-team`**: Where Site CRs and products are deployed

## Helm Installation

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace
```

## Contributing

- Use conventional commits (`feat:`, `fix:`, `docs:`, etc.)
- Run `just test` before committing
- See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines

## License

MIT License - see LICENSE file
