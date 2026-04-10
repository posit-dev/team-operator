---
title: Authentication Setup
description: Authentication configuration for Posit Team Operator including OIDC, SAML, and Keycloak
---

# Authentication Setup Guide

Configuring authentication is often the most environment-specific part of a Posit Team deployment. The right choice depends on what your organization already uses and what your security requirements are.

Team Operator supports three authentication types. **Password authentication** stores credentials locally in each product's database and requires no external dependencies. It is appropriate for development environments and quick proof-of-concept deployments, but it does not provide SSO and requires managing users separately in each product. **OIDC** (OpenID Connect) is the recommended choice for production. It integrates with standard identity providers — Okta, Azure AD/Entra ID, Auth0, Keycloak — and supports group-based role mapping. **SAML 2.0** is available for environments where OIDC is not an option, typically older enterprise identity providers.

Authentication is configured independently for Connect and Workbench. Both products can use the same IdP with different client registrations, or different IdPs if your organization requires it.

## Table of Contents

1. [Overview](#overview)
2. [Authentication Types](#authentication-types)
3. [OIDC Configuration](#oidc-configuration)
4. [SAML Configuration](#saml-configuration)
5. [Password Authentication](#password-authentication)
6. [Role-Based Access Control](#role-based-access-control)
7. [Keycloak Integration](#keycloak-integration)
8. [Secrets Management](#secrets-management)
9. [Troubleshooting](#troubleshooting)

## Overview

Team Operator uses the `AuthSpec` structure to configure authentication for Posit products. Authentication is configured per-product (Connect and Workbench) through the `auth` field in each product's spec.

### AuthSpec Structure

The complete `AuthSpec` type definition:

```go
type AuthSpec struct {
    Type               AuthType `json:"type,omitempty"`
    ClientId           string   `json:"clientId,omitempty"`
    Issuer             string   `json:"issuer,omitempty"`
    Groups             bool     `json:"groups,omitempty"`
    UsernameClaim      string   `json:"usernameClaim,omitempty"`
    EmailClaim         string   `json:"emailClaim,omitempty"`
    UniqueIdClaim      string   `json:"uniqueIdClaim,omitempty"`
    GroupsClaim        string   `json:"groupsClaim,omitempty"`
    DisableGroupsClaim bool     `json:"disableGroupsClaim,omitempty"`
    SamlMetadataUrl    string   `json:"samlMetadataUrl,omitempty"`
    SamlIdPAttributeProfile  string   `json:"samlIdPAttributeProfile,omitempty"`
    SamlUsernameAttribute    string   `json:"samlUsernameAttribute,omitempty"`
    SamlFirstNameAttribute   string   `json:"samlFirstNameAttribute,omitempty"`
    SamlLastNameAttribute    string   `json:"samlLastNameAttribute,omitempty"`
    SamlEmailAttribute       string   `json:"samlEmailAttribute,omitempty"`
    Scopes                   []string `json:"scopes,omitempty"`
    ViewerRoleMapping        []string `json:"viewerRoleMapping,omitempty"`
    PublisherRoleMapping     []string `json:"publisherRoleMapping,omitempty"`
    AdministratorRoleMapping []string `json:"administratorRoleMapping,omitempty"`
}
```

## Authentication Types

Team Operator supports three authentication types:

| Type | Value | Use Case |
|------|-------|----------|
| Password | `password` | Development, simple deployments |
| OpenID Connect (OIDC) | `oidc` | Enterprise SSO with OAuth2/OpenID Connect |
| Security Assertion Markup Language (SAML) | `saml` | Enterprise SSO with SAML 2.0 |

## OIDC Configuration

OpenID Connect is the recommended authentication method for production deployments. Before configuring OIDC in Team Operator, you need to register the application in your IdP, configure redirect URIs, and store the client secret in your secrets backend.

### Basic OIDC Configuration

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: production
  namespace: posit-team
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"
```

### Required IdP Settings

In your identity provider, create an OAuth2/OIDC application for each product. Configure the redirect URIs exactly as shown below — the paths are fixed by each product and cannot be changed:

- Connect: `https://connect.example.com/__login__/callback`
- Workbench: `https://workbench.example.com/oidc/callback`

Note the Client ID from the IdP registration (this goes in the spec) and generate a client secret (this goes in your secrets backend, not in the spec).

### Client Secret Configuration

The client secret must be stored in your secrets provider:

**For Kubernetes secrets:**
- Connect: `pub-client-secret` key
- Workbench: `dev-client-secret` key

**For AWS Secrets Manager:**
- Connect: `pub-client-secret` in your vault
- Workbench: `dev-client-secret` in your vault

### Claims Mapping

OIDC tokens carry user attributes as claims. The operator maps claims to user identity fields. If your IdP uses non-standard claim names, configure the mapping explicitly:

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"
      usernameClaim: "preferred_username"  # Claim for username
      emailClaim: "email"                  # Claim for email
      uniqueIdClaim: "sub"                 # Claim for unique identifier
```

**Default behavior:**
- If `emailClaim` is set but `uniqueIdClaim` is not, the email claim is used for unique ID
- Default `uniqueIdClaim` is `email`

### Group Claim Configuration

Group synchronization lets the operator read a user's group membership from the OIDC token and use it for role mapping. Enable it by adding `groups: true` and specifying the claim that carries group membership. You also need to request the appropriate scope so the IdP includes groups in the token:

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"
      groups: true                    # Enable group auto-provisioning
      groupsClaim: "groups"           # Claim containing group membership
      scopes:
        - "openid"
        - "profile"
        - "email"
        - "groups"                    # Scope to request groups
```

**Disabling the Groups Claim:**

Some IdPs support group-based auto-provisioning but do not include groups in the token. In that case, you can still enable group auto-provisioning while telling the operator not to read groups from the token:

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"
      groups: true                    # Still auto-provision groups
      disableGroupsClaim: true        # But don't try to read from token
```

### Custom Scopes

Override the default OIDC scopes:

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"
      scopes:
        - "openid"
        - "profile"
        - "email"
        - "offline_access"
```

### OIDC Examples by IdP

The configurations below are starting points for common identity providers. Replace placeholder values with your actual tenant IDs, client IDs, and domain names.

#### Okta

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "0oaxxxxxxxxx"
      issuer: "https://your-org.okta.com"
      usernameClaim: "preferred_username"
      emailClaim: "email"
      groups: true
      groupsClaim: "groups"
      scopes:
        - "openid"
        - "profile"
        - "email"
        - "groups"
```

#### Azure AD / Entra ID

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
      issuer: "https://login.microsoftonline.com/{tenant-id}/v2.0"
      usernameClaim: "preferred_username"
      emailClaim: "email"
      uniqueIdClaim: "oid"           # Azure object ID
      groups: true
      groupsClaim: "groups"
      scopes:
        - "openid"
        - "profile"
        - "email"
```

> **Note:** Azure AD requires specific application permissions to include group claims. Configure "Groups claim" in the Token configuration.

#### Auth0

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
      issuer: "https://your-tenant.auth0.com/"
      usernameClaim: "email"
      emailClaim: "email"
      groups: true
      groupsClaim: "https://your-namespace/groups"  # Custom claim namespace
      scopes:
        - "openid"
        - "profile"
        - "email"
```

#### Keycloak

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect"
      issuer: "https://keycloak.example.com/realms/posit"
      usernameClaim: "preferred_username"
      emailClaim: "email"
      groups: true
      groupsClaim: "groups"
      scopes:
        - "openid"
        - "profile"
        - "email"
        - "groups"
```

## SAML Configuration

SAML 2.0 authentication is available for enterprise environments where OIDC is not an option. Before configuring SAML, you need to register the Service Provider (SP) details in your IdP and obtain the metadata URL. The metadata URL must be reachable from within the cluster at runtime.

### Basic SAML Configuration

```yaml
spec:
  connect:
    auth:
      type: "saml"
      samlMetadataUrl: "https://idp.example.com/saml/metadata"
```

> **Required:** `samlMetadataUrl` must be set for SAML authentication.

### Attribute Profiles

SAML assertions use attribute URIs to carry user identity information, and different IdPs use different URI formats. Team Operator supports two approaches for mapping those attributes.

#### 1. Using IdP Attribute Profiles

Use a predefined attribute profile matching your IdP:

```yaml
spec:
  connect:
    auth:
      type: "saml"
      samlMetadataUrl: "https://idp.example.com/saml/metadata"
      samlIdPAttributeProfile: "azure"   # Options: default, azure, etc.
```

Built-in profiles:
- `default` - Standard SAML attributes
- `azure` - Microsoft Azure AD attributes

#### 2. Custom Attribute Mapping

Specify individual attribute URIs for complete control:

```yaml
spec:
  connect:
    auth:
      type: "saml"
      samlMetadataUrl: "https://idp.example.com/saml/metadata"
      samlUsernameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
      samlFirstNameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
      samlLastNameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
      samlEmailAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
```

> **Important:** `samlIdPAttributeProfile` and individual attribute mappings are mutually exclusive. The operator will return an error if both are specified.

### SAML Service Provider (SP) Configuration

Configure your IdP with these Service Provider details:

**Connect:**
- Entity ID: `https://connect.example.com/__login__`
- ACS URL: `https://connect.example.com/__login__/callback`

**Workbench:**
- Entity ID: `https://workbench.example.com/saml`
- ACS URL: `https://workbench.example.com/saml/acs`

### SAML Examples by IdP

#### Azure AD / Entra ID

```yaml
spec:
  connect:
    auth:
      type: "saml"
      samlMetadataUrl: "https://login.microsoftonline.com/{tenant-id}/federationmetadata/2007-06/federationmetadata.xml"
      samlIdPAttributeProfile: "azure"
```

Or with custom attributes:

```yaml
spec:
  connect:
    auth:
      type: "saml"
      samlMetadataUrl: "https://login.microsoftonline.com/{tenant-id}/federationmetadata/2007-06/federationmetadata.xml"
      samlUsernameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn"
      samlEmailAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
      samlFirstNameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
      samlLastNameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
```

#### Okta

```yaml
spec:
  connect:
    auth:
      type: "saml"
      samlMetadataUrl: "https://your-org.okta.com/app/xxxxxxxx/sso/saml/metadata"
      samlUsernameAttribute: "NameID"
      samlEmailAttribute: "email"
      samlFirstNameAttribute: "firstName"
      samlLastNameAttribute: "lastName"
```

### Workbench SAML Configuration

Workbench SAML uses the `usernameClaim` field for the username attribute:

```yaml
spec:
  workbench:
    auth:
      type: "saml"
      samlMetadataUrl: "https://idp.example.com/saml/metadata"
      usernameClaim: "email"   # Maps to auth-saml-sp-attribute-username
```

## Password Authentication

Password authentication stores credentials in each product's own database and requires no external IdP. It is the quickest way to get a deployment running, but it does not provide SSO, and user management must be done separately within each product. Reserve it for development and testing environments.

### Configuration

```yaml
spec:
  connect:
    auth:
      type: "password"
  workbench:
    auth:
      type: "password"
```

Password authentication is appropriate for development and testing, quick proof-of-concept deployments, and environments without enterprise SSO requirements. It stores credentials in the product's own database, does not provide SSO capabilities, and requires user management within each product. It is not recommended for production environments with security requirements.

## Role-Based Access Control

When using OIDC or SAML with group synchronization, the operator can automatically assign Connect roles based on IdP group membership. Users are assigned the highest matching role when they log in, so a user in both a publisher group and an admin group gets the administrator role.

### Connect Role Mappings

Connect supports three roles. Each role can map to multiple groups:

- **Viewer** - Can view published content
- **Publisher** - Can publish and manage content
- **Administrator** - Full administrative access

Configure role mappings in the auth block:

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"
      groups: true
      groupsClaim: "groups"
      viewerRoleMapping:
        - "connect-viewers"
        - "readonly-users"
      publisherRoleMapping:
        - "connect-publishers"
        - "data-scientists"
      administratorRoleMapping:
        - "connect-admins"
        - "platform-admins"
```

When a user logs in, Connect reads their group membership from the `groupsClaim` and checks it against each role mapping list. The user receives the highest matching role: Administrator takes precedence over Publisher, which takes precedence over Viewer. Users who match no mapping receive the default role configured separately in Connect.

Role mappings work the same way with SAML authentication, provided your IdP sends group membership in the SAML assertion.

### Workbench Role Mappings

Workbench uses admin groups for administrative access:

```yaml
spec:
  workbench:
    # Admin groups have access to the administrative dashboard
    adminGroups:
      - "workbench-admin"
      - "platform-admins"
    # Superuser groups have elevated administrative privileges
    adminSuperuserGroups:
      - "workbench-superusers"
```

### Default User Role

Set the default role for users who don't match any role mapping:

```yaml
spec:
  connect:
    config:
      Authorization:
        DefaultUserRole: "viewer"   # Options: viewer, publisher, administrator
```

## Keycloak Integration

Team Operator can deploy and manage a Keycloak instance directly within the cluster. This is useful when your organization does not have an existing IdP or when you want a self-contained deployment. When enabled, the operator provisions Keycloak with its own PostgreSQL database and ingress, and you then configure Connect and Workbench to use it as their OIDC provider.

### Enabling Keycloak

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: production
  namespace: posit-team
spec:
  keycloak:
    enabled: true
    image: "quay.io/keycloak/keycloak:latest"
    imagePullPolicy: IfNotPresent
```

### Keycloak Features

When enabled, Team Operator deploys a Keycloak instance in the namespace, provisions a PostgreSQL database for it, configures ingress routing to `key.<domain>`, and sets up the necessary service accounts and RBAC.

### Using Keycloak with Products

Configure products to use the deployed Keycloak:

```yaml
spec:
  keycloak:
    enabled: true
  connect:
    auth:
      type: "oidc"
      clientId: "connect"
      issuer: "https://key.example.com/realms/posit"
      groups: true
      groupsClaim: "groups"
```

### Keycloak Realm Configuration

After Keycloak deploys, you need to complete the initial setup through the admin console. Access it at `https://key.<domain>`, create a realm (for example, "posit"), create a client for each product, configure client credentials and redirect URIs, and set up user federation if you need LDAP or Active Directory integration.

## Secrets Management

Client secrets and tokens are never placed in the Site spec. Instead, the operator reads them from your configured secrets backend at reconciliation time. The keys in the secret must match the names listed below exactly — the operator looks up these specific keys.

### Kubernetes Secrets

For `secret.type: kubernetes`, create a secret with the required keys:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: site-secrets
  namespace: posit-team
type: Opaque
stringData:
  # Connect OIDC
  pub-client-secret: "your-connect-client-secret"

  # Workbench OIDC
  dev-client-secret: "your-workbench-client-secret"
  dev-admin-token: "generated-admin-token"
  dev-user-token: "generated-user-token"
```

### AWS Secrets Manager

For `secret.type: aws`, store secrets in AWS Secrets Manager:

| Secret Key | Description |
|------------|-------------|
| `pub-client-secret` | Connect OIDC client secret |
| `dev-client-secret` | Workbench OIDC client secret |
| `dev-admin-token` | Workbench admin authentication token |
| `dev-user-token` | Workbench user authentication token |

### Secret Structure Reference

| Product | Auth Type | Secret Key | Purpose |
|---------|-----------|------------|---------|
| Connect | OIDC | `pub-client-secret` | OAuth2 client secret |
| Workbench | OIDC | `dev-client-secret` | OAuth2 client secret |
| Workbench | OIDC | `dev-admin-token` | Admin API token |
| Workbench | OIDC | `dev-user-token` | User API token |

## Troubleshooting

### If you see "Invalid redirect URI" from your IdP

The redirect URI registered in the IdP does not match what Connect or Workbench sends. The paths are fixed — verify they are registered exactly as shown:

- Connect: `https://<connect-url>/__login__/callback`
- Workbench: `https://<workbench-url>/oidc/callback`

### If groups are not syncing from your IdP

Check that `groups: true` is set, that `groupsClaim` matches the claim name your IdP actually sends, and that the `groups` scope is included in `scopes`. Some IdPs require additional configuration to include group claims in tokens. Enable debug logging to see the raw claims:

```yaml
spec:
  connect:
    debug: true   # Enables OAuth2 logging
```

Then check the logs:

```bash
kubectl logs -n posit-team deploy/<site>-connect -f
```

### If users are not getting the correct roles

Group names in role mappings are case-sensitive and must match exactly what the IdP sends. Check that groups are included in the token (not truncated due to token size limits), and that `groupsClaim` matches the actual claim path. For IdPs using nested claims (for example, `realm_access.roles` in Keycloak), the claim path must reflect the nesting.

### If OIDC claims are not mapping to user attributes

Use [jwt.io](https://jwt.io) to decode a token from your IdP and inspect the actual claim names. Then update your spec to match:

```yaml
spec:
  connect:
    auth:
      usernameClaim: "preferred_username"  # Must exist in token
      emailClaim: "email"                  # Must exist in token
```

### If the SAML metadata URL is not accessible

The metadata URL must be reachable from within the cluster at runtime. Test it from a pod to rule out DNS or network policy issues:

```bash
kubectl run -it --rm curl-test --image=curlimages/curl --restart=Never -- curl -v <saml-metadata-url>
```

### If you see "IdPAttributeProfile Cannot Be Specified Together..."

This error means both `samlIdPAttributeProfile` and individual attribute fields are set. They are mutually exclusive — use one or the other:

```yaml
# Option 1: Use profile
samlIdPAttributeProfile: "azure"

# Option 2: Use individual attributes (mutually exclusive with profile)
samlUsernameAttribute: "..."
samlEmailAttribute: "..."
```

### If Workbench redirect URIs include port numbers

Workbench may append port 443 to redirect URIs in some configurations. The operator sets an `X-Rstudio-Request` header to prevent this. If you still see port numbers in redirect URIs, verify the Traefik middleware is correctly applied to the Workbench ingress.

### If Workbench users are not being provisioned on first login

Set `createUsersAutomatically: true` in the Workbench spec. Without this, Workbench requires a pre-existing system user account to match the authenticated identity:

```yaml
spec:
  workbench:
    createUsersAutomatically: true
```

## Complete Example

A complete Site configuration with OIDC authentication:

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: production
  namespace: posit-team
spec:
  domain: posit.example.com

  secret:
    type: "kubernetes"
    vaultName: "production-secrets"

  connect:
    image: ghcr.io/rstudio/rstudio-connect:ubuntu22-2024.10.0
    auth:
      type: "oidc"
      clientId: "connect-production"
      issuer: "https://login.microsoftonline.com/tenant-id/v2.0"
      usernameClaim: "preferred_username"
      emailClaim: "email"
      uniqueIdClaim: "oid"
      groups: true
      groupsClaim: "groups"
      scopes:
        - "openid"
        - "profile"
        - "email"
      viewerRoleMapping:
        - "Connect-Viewers"
      publisherRoleMapping:
        - "Connect-Publishers"
        - "Data-Scientists"
      administratorRoleMapping:
        - "Connect-Admins"

  workbench:
    image: ghcr.io/rstudio/rstudio-workbench:jammy-2024.12.0
    createUsersAutomatically: true
    auth:
      type: "oidc"
      clientId: "workbench-production"
      issuer: "https://login.microsoftonline.com/tenant-id/v2.0"
      usernameClaim: "preferred_username"
      scopes:
        - "openid"
        - "profile"
        - "email"
    adminGroups:
      - "Workbench-Admins"
    adminSuperuserGroups:
      - "Platform-Admins"
```

## Related Documentation

- [Product Team Site Management](./product-team-site-management.md) - Complete Site configuration guide
- [Posit Connect Admin Guide](https://docs.posit.co/connect/admin/) - Connect authentication documentation
- [Posit Workbench Admin Guide](https://docs.posit.co/ide/server-pro/admin/) - Workbench authentication documentation
