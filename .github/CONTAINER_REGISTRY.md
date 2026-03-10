# GitHub Container Registry Packages

This repository automatically builds and publishes container images to GitHub Container Registry (ghcr.io).

## Available Packages

### Service Images

| Package | Description | Pull Command |
|---------|-------------|--------------|
| `ghcr.io/${{ github.repository_owner }}/weather-station-ingestion` | Ingestion Service | `docker pull ghcr.io/${{ github.repository_owner }}/weather-station-ingestion:latest` |
| `ghcr.io/${{ github.repository_owner }}/weather-station-aggregation` | Aggregation Service | `docker pull ghcr.io/${{ github.repository_owner }}/weather-station-aggregation:latest` |
| `ghcr.io/${{ github.repository_owner }}/weather-station-query` | Query Service | `docker pull ghcr.io/${{ github.repository_owner }}/weather-station-query:latest` |
| `ghcr.io/${{ github.repository_owner }}/weather-station-discovery` | Discovery Service | `docker pull ghcr.io/${{ github.repository_owner }}/weather-station-discovery:latest` |

### Test Harness

| Package | Description | Pull Command |
|---------|-------------|--------------|
| `ghcr.io/${{ github.repository_owner }}/weather-station-test-harness` | Test Harness CLI | `docker pull ghcr.io/${{ github.repository_owner }}/weather-station-test-harness:latest` |

## Image Tags

Images are tagged with:
- `latest` - Latest build from main branch
- `v{version}` - Semantic version tags (e.g., `v1.0.0`)
- `{major}.{minor}` - Major.minor version (e.g., `1.0`)
- `{major}` - Major version only (e.g., `1`)
- `{branch-name}` - Branch name for development builds
- `{short-sha}` - Git commit SHA

## Usage

### Docker Compose

```yaml
version: '3.8'

services:
  ingestion:
    image: ghcr.io/${{ github.repository_owner }}/weather-station-ingestion:latest
    # ...
  
  aggregation:
    image: ghcr.io/${{ github.repository_owner }}/weather-station-aggregation:latest
    # ...
  
  query:
    image: ghcr.io/${{ github.repository_owner }}/weather-station-query:latest
    # ...
  
  discovery:
    image: ghcr.io/${{ github.repository_owner }}/weather-station-discovery:latest
    # ...
```

### Kubernetes with Helm

```bash
# Add the repository (if using GitHub Packages)
helm install weather-station charts/weather-station \
  --set ingestion.image.repository=ghcr.io/${{ github.repository_owner }}/weather-station-ingestion \
  --set aggregation.image.repository=ghcr.io/${{ github.repository_owner }}/weather-station-aggregation \
  --set query.image.repository=ghcr.io/${{ github.repository_owner }}/weather-station-query \
  --set discovery.image.repository=ghcr.io/${{ github.repository_owner }}/weather-station-discovery
```

### Test Harness

```bash
# Pull and run test-harness
docker run --rm ghcr.io/${{ github.repository_owner }}/weather-station-test-harness:latest --help

# Run tests against your deployment
docker run --rm ghcr.io/${{ github.repository_owner }}/weather-station-test-harness:latest grade --detailed

# Download NOAA data
docker run --rm \
  -v $(pwd)/data:/data \
  ghcr.io/${{ github.repository_owner }}/weather-station-test-harness:latest \
  retrieve --country de --limit 100 --output /data/csv
```

## Authentication

To pull images from GitHub Container Registry, you need to authenticate:

### Using Docker Login

```bash
echo $CR_PAT | docker login ghcr.io -u USERNAME --password-stdin
```

Where `CR_PAT` is a GitHub Personal Access Token with `read:packages` scope.

### Using Kubernetes Image Pull Secret

```bash
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=USERNAME \
  --docker-password=$CR_PAT \
  --namespace=weather-station
```

Then reference it in your Helm values:

```yaml
global:
  imagePullSecrets:
    - name: ghcr-secret
```

## Build Process

Images are automatically built and published by GitHub Actions:

1. **On every push to main** - Builds and tags with `latest` and commit SHA
2. **On version tags** - Builds and tags with semantic version
3. **Multi-platform** - Images are built for both `linux/amd64` and `linux/arm64`
4. **Caching** - Build layers are cached for faster subsequent builds

## Workflow Status

[![Build Services](https://github.com/${{ github.repository_owner }}/${{ github.repository }}/actions/workflows/build-services.yml/badge.svg)](https://github.com/${{ github.repository_owner }}/${{ github.repository }}/actions/workflows/build-services.yml)
[![Test Harness](https://github.com/${{ github.repository_owner }}/${{ github.repository }}/actions/workflows/test-harness.yml/badge.svg)](https://github.com/${{ github.repository_owner }}/${{ github.repository }}/actions/workflows/test-harness.yml)

## Image Sizes

Approximate image sizes (compressed):
- Ingestion: ~15 MB
- Aggregation: ~23 MB  
- Query: ~15 MB
- Discovery: ~15 MB
- Test Harness: ~20 MB

## Security Scanning

Images are automatically scanned for vulnerabilities using GitHub's built-in security features. Check the "Security" tab in the repository for vulnerability reports.

## Support

For issues or questions about these images, please open an issue in the GitHub repository.
