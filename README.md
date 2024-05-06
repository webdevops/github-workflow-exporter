# Github Workflows exporter

[![license](https://img.shields.io/github/license/webdevops/github-workflow-expoter.svg)](https://github.com/webdevops/github-workflow-expoter/blob/main/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fgithub--workflow--expoter-blue)](https://hub.docker.com/r/webdevops/github-workflow-expoter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fgithub--workflow--expoter-blue)](https://quay.io/repository/webdevops/github-workflow-expoter)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/github-workflow-expoter)](https://artifacthub.io/packages/search?repo=github-workflow-expoter)

Prometheus exporter for GitHub workflow runs

## Usage

```
Usage:
  github-workflow-exporter [OPTIONS]

Application Options:
      --log.debug                   debug mode [$LOG_DEBUG]
      --log.devel                   development mode [$LOG_DEVEL]
      --log.json                    Switch log output to json format [$LOG_JSON]
      --github.enterprise.url=      GitHub enterprise url (self hosted) [$GITHUB_ENTERPRISE_URL]
      --github.organization=        GitHub organization name [$GITHUB_ORGANIZATION]
      --github.token=               GitHub token auth: PAT [$GITHUB_TOKEN]
      --github.app.id=              GitHub app auth: App ID [$GITHUB_APP_ID]
      --github.app.installationid=  GitHub app auth: App installation ID [$GITHUB_APP_INSTALLATION_ID]
      --github.app.keyfile=         GitHub app auth: Private key (path to file) [$GITHUB_APP_PRIVATE_KEY]
      --github.workflows.timeframe= GitHub workflow timeframe for fetching (default: 168h) [$GITHUB_WORKFLOWS_TIMEFRAME]
      --scrape.time=                Scrape time (default: 30m) [$SCRAPE_TIME]
      --cache.path=                 Cache path (to folder, file://path... or azblob://storageaccount.blob.core.windows.net/containername or
                                    k8scm://{namespace}/{configmap}}) [$CACHE_PATH]
      --server.bind=                Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=        Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write=       Server write timeout (default: 10s) [$SERVER_TIMEOUT_WRITE]

Help Options:
  -h, --help                        Show this help message
```

### Authentication

Supports either PAT token auth via env var `GITHUB_TOKEN`
or GitHub App auth with env vars `GITHUB_APP_ID` (id), `GITHUB_APP_INSTALLATION_ID` (id) and `GITHUB_APP_PRIVATE_KEY` (file path).

## Metrics

| Metric                                         | Description                                   |
|------------------------------------------------|-----------------------------------------------|
| `github_workflow_latest_run`                   | Latest workflow run with conclusion as label  |
| `github_workflow_latest_run_timestamp_seconds` | Latest workflow run with timestamp as value   |
| `github_workflow_consecutive_failed_runs`      | Count of consecutive failed runs per workflow |
