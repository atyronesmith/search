# Configuration Guide

## Initial Setup

This application requires configuration of personal information and sensitive credentials. Follow these steps to set up your configuration:

### 1. Create Your Secrets Configuration

Copy the sample secrets file and customize it with your information:

```bash
cp secrets.cfg.sample secrets.cfg
```

Edit `secrets.cfg` with your personal information and credentials:

```ini
[user]
name = Your Name
email = your.email@example.com
home_dir = /Users/yourusername  # or /home/yourusername on Linux

[database]
password = your_secure_password_here
user = postgres

[api_keys]
dev_key = generate-a-secure-dev-key
prod_key = generate-a-secure-prod-key
```

**Important:** Never commit `secrets.cfg` to version control. It's already in `.gitignore`.

### 2. Configure Search Paths

The `search.cfg` file contains non-sensitive configuration. By default, it indexes:
- `~/Documents`
- `~/Downloads`

You can modify these paths in the `[indexing]` section:

```ini
[indexing]
watch_paths = ~/Documents,~/Downloads,~/Projects
```

### 3. Database Setup

The application uses PostgreSQL. Update your database credentials in `secrets.cfg`:

1. Database password in `[database]` section
2. Optionally set database user (defaults to `postgres`)

The full database URL will be constructed from these components.

### 4. API Keys and WebSocket Security

For production environments, you should enable authentication:

1. Generate secure API keys:
   ```bash
   openssl rand -base64 32  # For dev key
   openssl rand -base64 32  # For prod key
   openssl rand -base64 32  # For WebSocket token
   ```

2. Add them to `secrets.cfg`:
   ```ini
   [api_keys]
   dev_key = your-generated-dev-key
   prod_key = your-generated-prod-key
   ws_token = your-generated-ws-token
   ```

3. Configure CORS in `search.cfg`:
   ```ini
   [security]
   allowed_origins = http://localhost:3000,https://yourdomain.com
   require_ws_auth = true
   ```

If no API keys are configured, the system will run in open access mode (suitable for local development only).

## Security Best Practices

1. **Use strong passwords**: Generate secure passwords for database access
2. **Protect secrets.cfg**: Set appropriate file permissions:
   ```bash
   chmod 600 secrets.cfg
   ```
3. **Regular key rotation**: Periodically update API keys and passwords
4. **Environment variables**: For production, consider using environment variables instead of config files

## Troubleshooting

If the application can't find your configuration:

1. Check that `secrets.cfg` exists in the same directory as `search.cfg`
2. Verify file permissions allow the application to read the config
3. Check logs for specific configuration errors
4. Ensure paths use proper home directory notation (`~` or full paths)

## Example Production Setup

For production environments, use environment variables:

```bash
export DATABASE_URL="postgresql://user:pass@host:5432/dbname?sslmode=require"
export API_DEV_KEY="your-secure-dev-key"
export API_PROD_KEY="your-secure-prod-key"
```

These will override any file-based configuration.