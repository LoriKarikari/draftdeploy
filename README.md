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
  id-token: write

jobs:
  preview:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Deploy Preview
        uses: LoriKarikari/draftdeploy@v1
        with:
          azure-client-id: ${{ secrets.AZURE_CLIENT_ID }}
          azure-tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          azure-subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}
          azure-location: eastus
          compose-file: docker-compose.yml
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

## Setup

1. Create a service principal:
```bash
az ad sp create-for-rbac --name "draftdeploy" --role Contributor --scopes /subscriptions/YOUR_SUBSCRIPTION_ID
```

2. Configure OIDC federated credentials:
```bash
az ad app federated-credential create --id YOUR_APP_ID --parameters '{
  "name": "github-pr",
  "issuer": "https://token.actions.githubusercontent.com",
  "subject": "repo:YOUR_ORG/YOUR_REPO:pull_request",
  "audiences": ["api://AzureADTokenExchange"]
}'
```

3. Add GitHub secrets:

| Secret | Description |
|--------|-------------|
| `AZURE_CLIENT_ID` | Service principal app ID |
| `AZURE_TENANT_ID` | Azure AD tenant ID |
| `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |

No client secret needed - OIDC handles authentication securely.
