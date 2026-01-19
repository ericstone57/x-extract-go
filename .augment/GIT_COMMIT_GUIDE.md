# Git Commit Message Guide

**Conventional Commits standard for X-Extract Go project**

---

## 📋 Format

```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

---

## 🏷️ Types

| Type | Description | Example |
|------|-------------|---------|
| `feat` | New feature | `feat(api): add pause download endpoint` |
| `fix` | Bug fix | `fix(queue): prevent duplicate processing` |
| `docs` | Documentation only | `docs(readme): update installation steps` |
| `style` | Code style/formatting | `style(api): fix indentation` |
| `refactor` | Code change (no feature/fix) | `refactor(downloader): simplify retry logic` |
| `perf` | Performance improvement | `perf(queue): optimize pending query` |
| `test` | Adding/updating tests | `test(domain): add download state tests` |
| `chore` | Maintenance tasks | `chore(deps): update gin to v1.9.1` |
| `ci` | CI/CD changes | `ci(github): add test workflow` |

---

## 🎯 Scopes

Common scopes for this project:

| Scope | Description |
|-------|-------------|
| `api` | API handlers, router, middleware |
| `cli` | CLI commands and interface |
| `domain` | Domain models and logic |
| `app` | Application services (managers) |
| `infra` | Infrastructure (downloaders, repo, notifier) |
| `queue` | Queue management |
| `downloader` | Download execution |
| `config` | Configuration |
| `web` | Web UI |
| `docker` | Docker/deployment |
| `deps` | Dependencies |
| `test` | Testing infrastructure |

---

## ✅ Rules

1. **Subject line ≤ 50 characters**
   - Keep it concise and focused

2. **Use imperative mood**
   - ✅ "add feature" (imperative)
   - ❌ "added feature" (past tense)
   - ❌ "adds feature" (present tense)

3. **No period at end of subject**
   - ✅ `feat(api): add endpoint`
   - ❌ `feat(api): add endpoint.`

4. **Capitalize first letter of subject**
   - ✅ `feat(api): Add endpoint`
   - ❌ `feat(api): add endpoint`

5. **One logical change per commit**
   - Atomic commits make history cleaner
   - Easier to review and revert

6. **Use body for context (optional)**
   - Explain "what" and "why"
   - Not "how" (code shows that)

---

## 📝 Examples

### Good Examples ✅

```bash
# Feature addition
feat(api): add pause download endpoint

# Bug fix
fix(queue): prevent duplicate processing

# Documentation
docs(readme): update installation steps

# Refactoring
refactor(downloader): simplify retry logic

# Tests
test(domain): add download state tests

# Dependencies
chore(deps): update gin to v1.9.1

# Performance
perf(queue): optimize pending query with index

# With body
feat(api): add download priority support

Allows users to set priority (1-10) for downloads.
Higher priority downloads are processed first.
Adds new 'priority' field to Download entity.
```

### Bad Examples ❌

```bash
# Too vague
❌ Updated stuff
❌ Fixed bug
❌ Changes

# Not imperative
❌ Added new feature
❌ Fixing the queue bug

# Too long subject
❌ feat(api): add a new endpoint that allows users to pause downloads

# Missing type
❌ add pause endpoint

# Missing scope (when relevant)
❌ feat: add endpoint

# Work in progress (don't commit)
❌ WIP
❌ temp
❌ asdfasdf

# Too detailed in subject (use body)
❌ fix(queue): prevent duplicate processing by adding mutex lock
```

---

## 🔄 Common Patterns

### Adding Features
```bash
feat(api): add retry download endpoint
feat(cli): add stats command
feat(downloader): add YouTube support
feat(web): add download progress bar
```

### Fixing Bugs
```bash
fix(queue): prevent race condition in worker
fix(downloader): handle network timeout
fix(api): validate URL before adding
fix(config): expand environment variables
```

### Documentation
```bash
docs(readme): add Docker instructions
docs(api): update endpoint examples
docs(contributing): add commit guidelines
docs(augment): update project context
```

### Refactoring
```bash
refactor(app): extract download validation
refactor(api): simplify error handling
refactor(domain): rename Platform constants
```

### Tests
```bash
test(domain): add download lifecycle tests
test(app): add queue manager tests
test(integration): add end-to-end flow test
```

### Maintenance
```bash
chore(deps): update dependencies
chore(build): update Makefile targets
chore(docker): optimize image size
chore(gitignore): add IDE files
```

---

## 🚀 Quick Reference

### Template
```bash
git commit -m "<type>(<scope>): <subject>"
```

### Most Common
```bash
# Feature
git commit -m "feat(api): add new endpoint"

# Bug fix
git commit -m "fix(queue): resolve race condition"

# Documentation
git commit -m "docs(readme): update setup guide"

# Refactor
git commit -m "refactor(app): simplify logic"

# Test
git commit -m "test(domain): add unit tests"

# Chore
git commit -m "chore(deps): update packages"
```

---

## 💡 Tips

1. **Write commit message before coding**
   - Helps clarify what you're about to do
   - Keeps commits focused

2. **Use `git commit -v`**
   - Shows diff while writing message
   - Helps write better descriptions

3. **Amend last commit if needed**
   ```bash
   git commit --amend
   ```

4. **Use interactive rebase to clean up**
   ```bash
   git rebase -i HEAD~3
   ```

5. **Reference issues in footer**
   ```bash
   feat(api): add pause endpoint
   
   Closes #123
   ```

---

## 🔍 Verification

Before committing, ask yourself:

- [ ] Is the type correct?
- [ ] Is the scope appropriate?
- [ ] Is the subject ≤ 50 characters?
- [ ] Is it in imperative mood?
- [ ] Does it describe what changed?
- [ ] Is it a single logical change?

---

## 📚 Resources

- [Conventional Commits](https://www.conventionalcommits.org/)
- [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/)
- [Angular Commit Guidelines](https://github.com/angular/angular/blob/main/CONTRIBUTING.md#commit)

---

**End of Git Commit Guide**

