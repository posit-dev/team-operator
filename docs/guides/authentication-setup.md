# Authentication Setup Guide

This guide provides comprehensive documentation for configuring authentication in Posit Team Operator. Team Operator supports multiple authentication methods for both Posit Connect and Posit Workbench.

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
| OIDC | `oidc` | Enterprise SSO with OAuth2/OpenID Connect |
| SAML | `saml` | Enterprise SSO with SAML 2.0 |

## OIDC Configuration

OpenID Connect (OIDC) is the recommended authentication method for enterprise deployments.

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

Before configuring OIDC in Team Operator, you must configure your Identity Provider:

1. **Create an OAuth2/OIDC Application** in your IdP
2. **Configure Redirect URIs**:
   - Connect: `https://connect.example.com/__login__/callback`
   - Workbench: `https://workbench.example.com/oidc/callback`
3. **Note the Client ID** (provided in the spec)
4. **Generate a Client Secret** (stored in secrets)

### Client Secret Configuration

The client secret must be stored in your secrets provider:

**For Kubernetes secrets:**
- Connect: `pub-client-secret` key
- Workbench: `dev-client-secret` key

**For AWS Secrets Manager:**
- Connect: `pub-client-secret` in your vault
- Workbench: `dev-client-secret` in your vault

### Claims Mapping

Configure how OIDC claims map to user attributes:

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

Enable group synchronization from your IdP:

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

**Disabling Groups Claim:**

Some IdPs do not support a groups claim. To explicitly disable it:

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

SAML 2.0 authentication is supported for enterprise environments using SAML-based IdPs.

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

Team Operator supports two approaches for SAML attribute mapping:

#### 1. Using IdP Attribute Profiles

Use a predefined attribute profile that matches your IdP:

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

Your IdP needs to be configured with the following Service Provider details:

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

Password authentication is the simplest authentication method, suitable for development environments.

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

### When to Use Password Authentication

- Development and testing environments
- Quick proof-of-concept deployments
- Environments without enterprise SSO requirements

### Security Considerations

- Password authentication stores credentials in the product's database
- Not recommended for production environments with security requirements
- Does not provide SSO capabilities
- User management must be done within each product

## Role-Based Access Control

Team Operator supports automatic role mapping based on group membership from your IdP.

### Connect Role Mappings

Connect supports three roles:
- **Viewer** - Can view published content
- **Publisher** - Can publish and manage content
- **Administrator** - Full administrative access

Configure role mappings:

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

### How Role Mapping Works

1. When a user logs in, Connect reads their group membership from the `groupsClaim`
2. The user is assigned the highest matching role:
   - If any group matches `administratorRoleMapping` -> Administrator
   - Else if any group matches `publisherRoleMapping` -> Publisher
   - Else if any group matches `viewerRoleMapping` -> Viewer
   - Else -> Default role (configured separately)

### Role Mapping with SAML

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

Team Operator can deploy and manage a Keycloak instance for authentication.

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

When enabled, Team Operator:
- Deploys a Keycloak instance in the namespace
- Creates a PostgreSQL database for Keycloak
- Configures ingress routing to `key.<domain>`
- Sets up necessary service accounts and RBAC

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

After Keycloak is deployed, you'll need to:
1. Access Keycloak admin console at `https://key.<domain>`
2. Create a realm (e.g., "posit")
3. Create clients for each product
4. Configure client credentials and redirect URIs
5. Set up user federation if needed (LDAP, AD, etc.)

## Secrets Management

Authentication requires secrets to be properly configured in your secrets provider.

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

### Common OIDC Issues

#### 1. "Invalid redirect URI" Error

**Cause:** The redirect URI in the IdP doesn't match what the product sends.

**Solution:** Verify redirect URIs are configured exactly:
- Connect: `https://<connect-url>/__login__/callback`
- Workbench: `https://<workbench-url>/oidc/callback`

#### 2. Groups Not Syncing

**Cause:** Groups claim not configured or not included in token.

**Debug steps:**
1. Check if `groups: true` is set
2. Verify `groupsClaim` matches what your IdP sends
3. Ensure the `groups` scope is requested
4. Check if your IdP requires special configuration for group claims

**Enable OIDC logging for Connect:**
```yaml
spec:
  connect:
    debug: true   # Enables OAuth2 logging
```

#### 3. User Identity Issues

**Cause:** Claims mapping doesn't match IdP token.

**Solution:** Verify your IdP token contains the expected claims:
```yaml
spec:
  connect:
    auth:
      usernameClaim: "preferred_username"  # Must exist in token
      emailClaim: "email"                  # Must exist in token
```

### Common SAML Issues

#### 1. "Metadata URL Not Accessible"

**Cause:** The SAML metadata URL is unreachable from the cluster.

**Solutions:**
- Ensure the metadata URL is accessible from pods
- Check network policies allow outbound connections
- Verify DNS resolution works

#### 2. "IdPAttributeProfile Cannot Be Specified Together..."

**Cause:** Both `samlIdPAttributeProfile` and individual attributes are set.

**Solution:** Use one approach:
```yaml
# Option 1: Use profile
samlIdPAttributeProfile: "azure"

# Option 2: Use individual attributes
samlUsernameAttribute: "..."
samlEmailAttribute: "..."
```

#### 3. Attribute Mapping Not Working

**Debug steps:**
1. Check the SAML assertion from your IdP
2. Verify attribute names match exactly (case-sensitive)
3. Use full URIs for standard attributes:
   ```yaml
   samlUsernameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn"
   ```

### Debugging Token Claims

To debug OIDC token claims:

1. **Enable Debug Logging:**
   ```yaml
   spec:
     connect:
       debug: true
   ```

2. **Check Pod Logs:**
   ```bash
   kubectl logs -n posit-team deploy/<site>-connect -f
   ```

3. **Decode JWT Tokens:**
   Use [jwt.io](https://jwt.io) to inspect tokens and verify claims.

### Group Membership Issues

If users aren't getting the correct roles:

1. **Verify group claim is present:**
   - Check the `groupsClaim` field matches your IdP
   - Some IdPs use nested claims (e.g., `realm_access.roles`)

2. **Check group name matching:**
   - Group names in role mappings must match exactly
   - Group names are case-sensitive

3. **Verify IdP configuration:**
   - Ensure groups are included in the token
   - Check token size limits (large group lists may be truncated)

### Workbench-Specific Issues

#### OIDC Callback URL Issues

Workbench may include port numbers in redirect URIs. The operator sets a header to prevent this:
```yaml
X-Rstudio-Request: https://<workbench-url>
```

If you see port 443 in redirect URIs, ensure Traefik middleware is correctly applied.

#### User Provisioning

For Workbench with OIDC/SAML:
```yaml
spec:
  workbench:
    createUsersAutomatically: true  # Create system users on first login
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
