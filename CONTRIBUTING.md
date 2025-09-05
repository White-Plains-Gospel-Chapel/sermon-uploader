# 🤝 Contributing to Sermon Uploader

> **TL;DR**: Fork → Branch → Code → Test → PR. Be nice, write tests, follow the style.

## 🎯 Quick Links

- [💬 **Discussions**](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions) - Ask questions
- [📚 **Wiki**](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/wiki) - Read docs
- [🐛 **Issues**](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/issues) - Report bugs
- [🚀 **Quick Start**](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/wiki/Developer-Setup) - Dev setup

## 🎨 Ways to Contribute

### 💬 Not a Coder? You Can Still Help!

- **Answer questions** in [Discussions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions)
- **Report bugs** with detailed information
- **Suggest features** in [Ideas discussions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/ideas)
- **Improve documentation** - fix typos, clarify instructions
- **Share your setup** in [Show and Tell](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/show-and-tell)

### 💻 For Developers

- **Fix bugs** from the [issue tracker](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/issues)
- **Implement features** from [Ideas discussions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/ideas)
- **Improve performance** - especially for Raspberry Pi
- **Add tests** - we always need more test coverage
- **Enhance documentation** - code examples, API docs

## 🚀 Getting Started

### Step 1: Fork & Clone (2 minutes)

```bash
# Fork on GitHub, then:
git clone git@github.com:YOUR-USERNAME/sermon-uploader.git
cd sermon-uploader
git remote add upstream git@github.com:White-Plains-Gospel-Chapel/sermon-uploader.git
```

### Step 2: Set Up Development Environment (10 minutes)

See [Developer Setup Guide](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/wiki/Developer-Setup)

Quick version:
```bash
# Backend (Go)
cd backend
go mod download
go run main.go

# Frontend (React)
cd frontend
npm install
npm run dev
```

### Step 3: Create a Branch (30 seconds)

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

## 📝 Coding Guidelines

### 🎨 Style Guide

**Go Backend:**
- Run `go fmt` before committing
- Run `go vet` to catch issues
- Follow [Effective Go](https://golang.org/doc/effective_go)

**React Frontend:**
- Run `npm run lint` before committing
- Use TypeScript types (no `any`)
- Components in PascalCase

**General:**
- Clear variable names > clever names
- Comments for "why", not "what"
- Keep functions small and focused

### 🧪 Testing Requirements

- **Add tests** for new features
- **Fix tests** if you break them
- **Run tests** before pushing:

```bash
# Backend
cd backend && go test ./...

# Frontend
cd frontend && npm test
```

### 📚 Documentation

When adding features, update:
- [ ] Code comments
- [ ] API documentation
- [ ] Wiki if needed
- [ ] README if significant

For documentation changes, follow [AI_DOCUMENTATION_GUIDE.md](docs/AI_DOCUMENTATION_GUIDE.md)

## 🔄 Pull Request Process

### 1. Before Creating PR

- [ ] Tests pass locally
- [ ] Code follows style guide
- [ ] Branch is up to date with main
- [ ] Commit messages are clear

### 2. PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation
- [ ] Performance improvement

## Testing
- [ ] Tests pass locally
- [ ] Tested on Raspberry Pi
- [ ] Tested with Docker

## Screenshots (if applicable)
[Add screenshots]

## Related Issues
Fixes #123
```

### 3. After Creating PR

- Be responsive to review feedback
- Make requested changes promptly
- Don't force push after reviews start
- Be patient - reviews take time

## 🎯 What Makes a Good PR?

### ✅ Good PRs

- **Focused**: One feature/fix per PR
- **Tested**: Includes tests, passes CI
- **Documented**: Clear description and comments
- **Clean**: No unrelated changes
- **Small**: <500 lines when possible

### ❌ PRs We Can't Merge

- No tests for new features
- Breaks existing functionality
- Massive PRs without discussion
- Code style violations
- Unrelated changes mixed in

## 🐛 Reporting Issues

### Good Bug Reports Include

```markdown
**Environment:**
- OS: [Mac/Pi/Docker]
- Version: [0.2.0]
- Component: [Frontend/Backend]

**Steps to Reproduce:**
1. Do this
2. Then this
3. See error

**Expected:** What should happen
**Actual:** What actually happens

**Logs:**
```
Error messages
```
```

## 💡 Proposing Features

Before coding big features:

1. **Discuss first** in [Ideas](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/ideas)
2. **Get feedback** from maintainers
3. **Agree on approach** before coding
4. **Start small** with MVP

## 🤝 Code of Conduct

### Be Nice

- 🤝 Be respectful and inclusive
- 💬 Be constructive with feedback
- 🎯 Focus on what's best for users
- 🚫 No harassment or discrimination

### Be Professional

- ✅ Accept feedback gracefully
- 🔄 Be open to different approaches
- 📚 Help others learn
- 🎉 Celebrate contributions

## 🏆 Recognition

Contributors are recognized in:
- Release notes
- README contributors section
- Discussion highlights
- Special thanks in major releases

## ❓ Questions?

- **Technical questions**: [Q&A Discussions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/q-a)
- **Feature ideas**: [Ideas Discussions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/ideas)
- **Bug reports**: [Issues](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/issues)

## 📜 License

By contributing, you agree that your contributions will be licensed under the same license as the project.

---

**Thank you for contributing!** Every contribution, big or small, makes a difference. 🙏