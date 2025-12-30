# Documentation Index

This directory contains comprehensive documentation for the CLIProxyAPI Plus project.

## 📚 Quick Navigation

### 🔧 Getting Started
- **[Quick Start Testing](QUICK_START_TESTING.md)** - Test the Gemini CLI "ALL" fix in 5 minutes
- **[Development Scripts Guide](DEV_SCRIPTS.md)** - Using dev.bat/dev.sh for development
- **[Troubleshooting Guide](TROUBLESHOOTING.md)** - Common issues and solutions

### 🐛 Gemini CLI "ALL" Fix
- **[Complete Summary](COMPLETE_SUMMARY.md)** - Executive overview of all work completed
- **[Fix Summary](FIX_SUMMARY.md)** - High-level fix description
- **[Technical Details](GEMINI_ALL_FIX.md)** - Detailed technical documentation
- **[Test Plan](TEST_PLAN.md)** - 18 test cases for validation

### 🐳 Docker & CI/CD
- **[Docker Branch Build Guide](DOCKER_BRANCH_BUILD.md)** - Using development branch images
- **[GitHub Actions Setup](GITHUB_ACTIONS_SETUP.md)** - Configuring automated builds

### 📖 SDK Documentation
- **[SDK Usage Guide](sdk-usage.md)** ([中文](sdk-usage_CN.md)) - Basic SDK integration
- **[SDK Access Guide](sdk-access.md)** ([中文](sdk-access_CN.md)) - Authentication and access
- **[SDK Advanced Guide](sdk-advanced.md)** ([中文](sdk-advanced_CN.md)) - Advanced features
- **[SDK Watcher Guide](sdk-watcher.md)** ([中文](sdk-watcher_CN.md)) - File watching

---

## 📋 Documentation by Category

### For End Users

| Document | Description | When to Use |
|----------|-------------|-------------|
| [Quick Start Testing](QUICK_START_TESTING.md) | Test the fix quickly | Want to try the fix immediately |
| [Troubleshooting Guide](TROUBLESHOOTING.md) | Common issues | Encountering problems |
| [Docker Branch Build](DOCKER_BRANCH_BUILD.md) | Use Docker images | Prefer Docker over building |

### For Developers

| Document | Description | When to Use |
|----------|-------------|-------------|
| [Development Scripts](DEV_SCRIPTS.md) | Dev tools | Local development |
| [Technical Details](GEMINI_ALL_FIX.md) | Code analysis | Understanding the fix |
| [Test Plan](TEST_PLAN.md) | Testing strategy | Writing/running tests |
| [GitHub Actions Setup](GITHUB_ACTIONS_SETUP.md) | CI/CD config | Setting up automation |

### For Project Managers

| Document | Description | When to Use |
|----------|-------------|-------------|
| [Complete Summary](COMPLETE_SUMMARY.md) | Full overview | Understanding scope |
| [Fix Summary](FIX_SUMMARY.md) | Executive summary | Quick briefing |

### For SDK Users

| Document | Description | When to Use |
|----------|-------------|-------------|
| [SDK Usage](sdk-usage.md) | Basic integration | Getting started with SDK |
| [SDK Access](sdk-access.md) | Authentication | Setting up auth |
| [SDK Advanced](sdk-advanced.md) | Advanced features | Complex use cases |
| [SDK Watcher](sdk-watcher.md) | File watching | Monitoring changes |

---

## 🎯 Common Scenarios

### "I want to test the Gemini CLI fix"
1. Start with [Quick Start Testing](QUICK_START_TESTING.md)
2. If issues arise, check [Troubleshooting Guide](TROUBLESHOOTING.md)
3. For technical details, see [Technical Details](GEMINI_ALL_FIX.md)

### "I want to build Docker images from my branch"
1. Read [GitHub Actions Setup](GITHUB_ACTIONS_SETUP.md) for configuration
2. Use [Docker Branch Build](DOCKER_BRANCH_BUILD.md) for usage
3. Check [Complete Summary](COMPLETE_SUMMARY.md) for overview

### "I want to develop locally"
1. Follow [Development Scripts](DEV_SCRIPTS.md)
2. Run tests using [Test Plan](TEST_PLAN.md)
3. Check [Troubleshooting Guide](TROUBLESHOOTING.md) if needed

### "I want to integrate the SDK"
1. Start with [SDK Usage](sdk-usage.md)
2. Configure auth with [SDK Access](sdk-access.md)
3. Explore [SDK Advanced](sdk-advanced.md) for more features

---

## 📊 Documentation Statistics

- **Total Files**: 17
- **Total Size**: ~140 KB
- **Languages**: English, Chinese
- **Categories**: 4 (Getting Started, Fix, Docker, SDK)
- **Test Cases**: 18
- **Code Examples**: 50+

---

## 🔄 Recent Updates

### 2024-12-31
- ✅ Added Complete Summary
- ✅ Added Quick Start Testing guide
- ✅ Added Docker Branch Build guide
- ✅ Added GitHub Actions Setup guide
- ✅ Updated Troubleshooting guide (marked fix as complete)
- ✅ Created comprehensive Test Plan

---

## 📝 Documentation Standards

### File Naming
- Use `UPPERCASE_WITH_UNDERSCORES.md` for major documents
- Use `lowercase-with-dashes.md` for SDK guides
- Add `_CN.md` suffix for Chinese versions

### Structure
- Start with clear title and overview
- Use emoji for visual navigation (optional)
- Include table of contents for long documents
- Provide code examples
- Add troubleshooting sections
- Include "Next Steps" or "Related Documentation"

### Code Examples
- Use proper syntax highlighting
- Include comments for clarity
- Show both success and error cases
- Provide complete, runnable examples

---

## 🤝 Contributing to Documentation

### Adding New Documentation
1. Follow the naming conventions
2. Add entry to this index
3. Link from related documents
4. Update README.md if user-facing

### Updating Existing Documentation
1. Keep the structure consistent
2. Update "Last Updated" date
3. Add to "Recent Updates" section
4. Verify all links still work

### Translation
1. Create `_CN.md` version
2. Keep structure identical to English
3. Translate all content, including code comments
4. Update index with both versions

---

## 🔗 External Resources

### Official Documentation
- [Go Documentation](https://go.dev/doc/)
- [Docker Documentation](https://docs.docker.com/)
- [GitHub Actions](https://docs.github.com/en/actions)

### Related Projects
- [CLIProxyAPI (Mainline)](https://github.com/router-for-me/CLIProxyAPI)
- [Gemini CLI](https://github.com/google/generative-ai-cli)

### Community
- [GitHub Issues](https://github.com/your-org/CLIProxyAPIPlus/issues)
- [GitHub Discussions](https://github.com/your-org/CLIProxyAPIPlus/discussions)

---

## 📞 Getting Help

### Documentation Issues
If you find errors or have suggestions:
1. Check if issue already exists
2. Create new issue with "documentation" label
3. Provide specific page and section
4. Suggest improvements

### Technical Support
For technical issues:
1. Check [Troubleshooting Guide](TROUBLESHOOTING.md)
2. Search existing issues
3. Create new issue with details
4. Include logs and environment info

---

## 📅 Maintenance

### Regular Updates
- Review and update quarterly
- Check for broken links monthly
- Update screenshots when UI changes
- Refresh code examples with new versions

### Version Tracking
- Major updates: Increment version in CHANGELOG.md
- Minor updates: Update "Last Updated" date
- Track changes in git history

---

**Last Updated**: 2024-12-31
**Total Documents**: 17
**Maintained By**: CLIProxyAPI Plus Team
