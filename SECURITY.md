# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in Sercha Core, please report it responsibly.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to:

**security@sercha.dev**

Include the following information in your report:

- Type of vulnerability (e.g., SQL injection, XSS, authentication bypass)
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability

### Response Timeline

- **Initial Response**: Within 48 hours of receiving your report
- **Status Update**: Within 7 days with an assessment
- **Resolution**: We aim to release a fix within 30 days for critical vulnerabilities

### What to Expect

1. **Acknowledgment**: We will acknowledge receipt of your report
2. **Assessment**: We will assess the vulnerability and determine its severity
3. **Communication**: We will keep you informed of our progress
4. **Credit**: We will credit you in our release notes (unless you prefer anonymity)

## Security Best Practices

When deploying Sercha Core, please follow these security best practices:

### Environment Variables

- **JWT_SECRET**: Use a strong, random secret (32+ characters)
- **DATABASE_URL**: Use strong passwords and restrict network access
- Never commit secrets to version control

### Network Security

- Deploy behind a reverse proxy (nginx, Traefik) with TLS
- Use firewall rules to restrict access to internal services
- PostgreSQL and Vespa should not be publicly accessible

### Authentication

- Configure OAuth2 providers with appropriate scopes
- Regularly rotate OAuth client secrets
- Use short-lived JWT tokens

### Data Protection

- Enable TLS for all external connections
- Encrypt sensitive data at rest in PostgreSQL
- Regularly backup your database

### Docker Security

- Use specific image tags, not `latest` in production
- Run containers as non-root users
- Keep base images updated

## Security Features

Sercha Core includes the following security features:

- **JWT Authentication**: Stateless token-based authentication
- **OAuth2 Integration**: Secure third-party authentication
- **Role-Based Access**: Team and admin role separation
- **Input Validation**: Request validation on all endpoints
- **SQL Injection Prevention**: Parameterized queries via GORM
- **XSS Prevention**: Content-Type enforcement and sanitization
- **CORS Configuration**: Configurable allowed origins

## Vulnerability Disclosure Policy

We follow a coordinated disclosure process:

1. Reporter submits vulnerability privately
2. We acknowledge and assess the report
3. We develop and test a fix
4. We release the fix and publish an advisory
5. Reporter may publish details 30 days after the fix is released

We appreciate your help in keeping Sercha Core and its users secure.
