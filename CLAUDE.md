# Team Operator

Kubernetes operator for managing Posit Team deployments.

## Project Structure

- **`api/`**: Kubernetes API/CRD definitions
- **`cmd/`**: Main operator entry point
- **`internal/`**: Core operator logic and controllers
- **`config/`**: Kubernetes manifests and Kustomize configurations
- **`dist/chart/`**: Helm chart for deployment
- **`flightdeck/`**: Landing page dashboard component
- **`client-go/`**: Generated Kubernetes client code
- **`pkg/`**: Shared packages

## Build and Development

```bash
just build          # Build operator binary
just test           # Run tests
just run            # Run operator locally
just format         # Format code
just helm-lint      # Lint Helm chart
just helm-template  # Render Helm templates
```

## Helm Installation

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace
```

## Contributing

- Use conventional commits (`feat:`, `fix:`, `docs:`, etc.)
- Run `just format` before committing
- Ensure tests pass with `just test`

## License

MIT License - see LICENSE file
