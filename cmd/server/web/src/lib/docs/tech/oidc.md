# OIDC / SSO

Damask supports single sign-on via any OpenID Connect provider.

## Configuration

```
OIDC_ISSUER_URL=https://auth.example.com/realms/myrealm
OIDC_CLIENT_ID=damask
OIDC_CLIENT_SECRET=your-client-secret
```

The redirect URI to register with your provider:

```
https://dam.example.com/auth/oidc/callback
```

## Keycloak

1. Create a new client in Keycloak with client type **OpenID Connect**
2. Set **Valid redirect URIs** to `https://dam.example.com/auth/oidc/callback`
3. Copy the client secret from the **Credentials** tab
4. Set `OIDC_ISSUER_URL` to `https://keycloak.example.com/realms/<realm>`

## Authelia

```yaml
# authelia configuration.yml
identity_providers:
  oidc:
    clients:
      - client_id: damask
        client_secret: your-secret-hash
        redirect_uris:
          - https://dam.example.com/auth/oidc/callback
        scopes: [openid, profile, email]
```

## Authentik

1. Create an **OAuth2/OIDC Provider** in Authentik
2. Set redirect URI to `https://dam.example.com/auth/oidc/callback`
3. Create an **Application** linked to the provider
4. Copy the **Client ID** and **Client Secret**

## Account linking

Existing Damask accounts are linked to OIDC by email address on first SSO login. Users can unlink via **Settings → Account**.
