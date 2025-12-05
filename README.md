# DraftDeploy

Deploy docker-compose stacks to Azure Container Instances for PR previews.

## Usage

```yaml
name: Preview Deploy

on:
  pull_request:
    types: [opened, synchronize, reopened, closed]

jobs:
  preview:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      id-token: write
    steps:
      - uses: actions/checkout@v4

      - name: Azure Login
        uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Deploy Preview
        uses: LoriKarikari/draftdeploy@v1
        with:
          azure-subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}
          azure-location: eastus
          compose-file: docker-compose.yml
          github-token: ${{ secrets.GITHUB_TOKEN }}
```
