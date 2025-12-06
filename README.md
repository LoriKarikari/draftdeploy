# DraftDeploy

Deploy docker-compose stacks to Azure Container Instances for PR previews.

## Usage

```yaml
name: Preview Deploy

on:
  pull_request:
    types: [opened, synchronize, reopened, closed]

permissions:
  contents: read
  pull-requests: write

jobs:
  preview:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Deploy Preview
        uses: LoriKarikari/draftdeploy@v1
        env:
          AZURE_CLIENT_ID: ${{ secrets.AZURE_CLIENT_ID }}
          AZURE_TENANT_ID: ${{ secrets.AZURE_TENANT_ID }}
          AZURE_CLIENT_SECRET: ${{ secrets.AZURE_CLIENT_SECRET }}
        with:
          azure-subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}
          azure-location: eastus
          compose-file: docker-compose.yml
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

## Required Secrets

| Secret | Description |
|--------|-------------|
| `AZURE_CLIENT_ID` | Service principal app ID |
| `AZURE_TENANT_ID` | Azure AD tenant ID |
| `AZURE_CLIENT_SECRET` | Service principal password |
| `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |

Create a service principal:
```bash
az ad sp create-for-rbac --name "draftdeploy" --role Contributor --scopes /subscriptions/YOUR_SUBSCRIPTION_ID
```
