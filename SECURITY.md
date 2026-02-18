# Security Policy

## Overview

The Kanban Society takes security seriously. This document outlines security considerations and best practices for using and contributing to this project.

## Security Improvements Made

### Path Traversal Protection (Fixed)

**Issue**: File operations were vulnerable to path traversal attacks where malicious filenames could write outside intended directories.

**Fix**: 
- Added path validation in `artifacts.go` using `filepath.Clean()` and `filepath.Base()`
- Verification that final paths remain within target directories
- Rejection of filenames containing `..`, `.`, or empty names

### File Permissions Hardening (Fixed)

**Issue**: Files and directories were created with overly permissive access rights.

**Fix**:
- Changed file permissions from `0644` to `0640` (owner read/write, group read only)
- Changed directory permissions from `0755` to `0750` (owner full, group read/execute)
- Reduces risk of unauthorized access to sensitive data

### Input Validation

**Status**: Implemented

The application validates and sanitizes user input in several areas:
- Debate topics are sanitized before use in filenames
- File paths are cleaned and validated
- CLI commands use parameterized execution (not shell strings)

## Known Security Considerations

### CLI Provider Flags

The Claude CLI provider uses the `--dangerously-skip-permissions` flag, which bypasses normal permission checks in the Claude CLI tool. This is intentional for automation but users should be aware:

**Risk**: The Claude CLI will execute without user confirmation prompts
**Mitigation**: Only use in trusted environments and review tasks before execution
**Recommendation**: Consider removing this flag in production deployments

### API Key Management

**Best Practices**:
1. Store API keys in environment variables, never in code or config files
2. Use separate API keys for development and production
3. Rotate API keys regularly
4. Use CLI providers when possible to avoid managing API keys

**Environment Variables Required**:
- `ANTHROPIC_API_KEY` - For Claude API access
- `OPENAI_API_KEY` - For GPT API access
- `GOOGLE_API_KEY` - For Gemini API access
- Additional keys for other providers as needed

### Network Security

**TLS/HTTPS**: All API providers use HTTPS for communication, ensuring data is encrypted in transit.

**Local Providers**: Ollama and LM Studio use HTTP by default as they run locally. Ensure these are not exposed to untrusted networks.

## Reporting Security Issues

If you discover a security vulnerability, please report it responsibly:

1. **Do NOT** open a public GitHub issue
2. Email the maintainers directly (check repository for contact info)
3. Include detailed steps to reproduce the issue
4. Allow reasonable time for a fix before public disclosure

## Security Update Process

1. Security issues are prioritized and fixed immediately
2. Fixes are tested thoroughly before release
3. Security advisories are published for significant issues
4. Users are notified through GitHub releases and security advisories

## Security Best Practices for Users

### Running in Production

1. **Use restrictive file permissions**: Ensure output directories have appropriate access controls
2. **Validate input**: Review task descriptions and prompts before execution
3. **Sandbox execution**: Consider running in containers or VMs
4. **Monitor logs**: Review generated content and execution logs
5. **Limit network access**: Restrict outbound connections if possible

### Development and Testing

1. **Use test API keys**: Never use production keys in development
2. **Review generated code**: Always review AI-generated code before execution
3. **Test in isolation**: Use separate environments for testing
4. **Keep dependencies updated**: Regularly update Go modules

### Data Privacy

1. **Sensitive data**: Avoid including secrets, passwords, or PII in prompts
2. **Output review**: Review all generated artifacts before sharing
3. **API providers**: Understand the data retention policies of AI providers
4. **Local alternatives**: Use Ollama/LM Studio for sensitive workflows

## Compliance Considerations

### Data Processing

When using cloud AI providers (Claude, GPT, Gemini), be aware that:
- Prompts and responses may be processed in third-party data centers
- Different providers have different data retention policies
- Some providers may use data for model improvement (check terms of service)
- Consider using local models (Ollama) for sensitive data

### Audit Trail

The application generates:
- Session summaries with timestamps
- Artifact manifests tracking file creation
- Checkpoint files for workflow state
- Debug logs (when enabled)

Retain these for compliance and audit purposes.

## Security Checklist for Contributors

When contributing code, ensure:

- [ ] No hardcoded secrets or API keys
- [ ] Input validation for user-provided data
- [ ] Path sanitization for file operations
- [ ] Appropriate error handling without information leakage
- [ ] Secure defaults for permissions
- [ ] Dependencies are up to date
- [ ] No unsafe deserialization
- [ ] SQL injection prevention (if applicable)
- [ ] XSS prevention in any web interfaces
- [ ] CSRF protection for web endpoints

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Go Security Best Practices](https://go.dev/doc/security/best-practices)
- [CWE - Common Weakness Enumeration](https://cwe.mitre.org/)

---

Last Updated: 2026-02-05
