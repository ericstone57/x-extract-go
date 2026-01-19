# .augment - Project Context for AI Assistants

This folder contains comprehensive project context documentation designed for AI coding assistants (like Augment, Cursor, GitHub Copilot, etc.) to quickly understand the codebase when switching machines or starting new sessions.

---

## 📁 Files in This Folder

### 1. **PROJECT_CONTEXT.md**
**Complete project overview and reference guide**

Contains:
- Project overview and purpose
- Architecture (Clean Architecture pattern)
- Tech stack and dependencies
- Project structure
- Configuration details
- Entry points (server, CLI)
- API endpoints
- Key workflows and lifecycles
- Build & run instructions
- Development guidelines
- Common tasks
- Troubleshooting
- Quick reference

**Use this when**: Starting a new session, onboarding, or need high-level understanding.

---

### 2. **CODEBASE_MAP.md**
**Detailed code navigation and structure**

Contains:
- Core domain models with code snippets
- Application services (DownloadManager, QueueManager)
- Infrastructure layer (downloaders, repository, notifier)
- API layer (router, handlers, middleware)
- Utilities (logger, validator)
- Data flow examples
- Testing structure
- Configuration loading
- Dependency injection patterns

**Use this when**: Need to find specific code, understand data flow, or navigate the codebase.

---

### 3. **DEVELOPMENT_NOTES.md**
**Working notes and development patterns**

Contains:
- Current implementation state
- Known limitations
- Design decisions and rationale
- Code patterns (error handling, logging, etc.)
- Common development tasks (step-by-step)
- Testing guidelines
- Debugging tips
- Performance considerations
- Future enhancement ideas

**Use this when**: Making changes, debugging, or extending functionality.

---

### 4. **GIT_COMMIT_GUIDE.md**
**Git commit message standards**

Contains:
- Conventional Commits format
- Commit types and scopes
- Rules and best practices
- Good and bad examples
- Common patterns
- Quick reference templates
- Verification checklist

**Use this when**: Writing commit messages.

---

## 🚀 Quick Start for AI Assistants

When starting a new conversation or switching machines:

1. **Read PROJECT_CONTEXT.md first** - Get the big picture
2. **Reference CODEBASE_MAP.md** - Find specific code locations
3. **Check DEVELOPMENT_NOTES.md** - Understand patterns and decisions

---

## 🔄 Keeping Context Updated

These files should be updated when:
- Major architectural changes occur
- New features are added
- Configuration structure changes
- New dependencies are introduced
- Design decisions are made

**Update frequency**: After significant changes, not every commit.

---

## 📝 Usage Examples

### Example 1: "Add support for YouTube downloads"
1. Check **PROJECT_CONTEXT.md** → "Architecture" section
2. Read **DEVELOPMENT_NOTES.md** → "Adding a New Download Platform"
3. Use **CODEBASE_MAP.md** → Find downloader interfaces and examples

### Example 2: "Why is the queue not processing?"
1. Check **DEVELOPMENT_NOTES.md** → "Debugging Tips" → "Queue not processing"
2. Reference **PROJECT_CONTEXT.md** → "Troubleshooting"
3. Use **CODEBASE_MAP.md** → "QueueManager" to understand flow

### Example 3: "How do I add a new API endpoint?"
1. Read **DEVELOPMENT_NOTES.md** → "Adding a New API Endpoint"
2. Reference **CODEBASE_MAP.md** → "API Layer" for examples
3. Check **PROJECT_CONTEXT.md** → "API Endpoints" for conventions

### Example 4: "How should I write my commit message?"
1. Read **GIT_COMMIT_GUIDE.md** → Complete guide
2. Check **CHEATSHEET.md** → "Git Commit Messages" for quick reference
3. Follow the format: `<type>(<scope>): <subject>`

---

## 🎯 Benefits

### For AI Assistants
- **Faster context loading** - No need to scan entire codebase
- **Better suggestions** - Understand patterns and conventions
- **Consistent code** - Follow established patterns
- **Reduced errors** - Know limitations and gotchas

### For Developers
- **Machine portability** - Same context on any machine
- **Session continuity** - Pick up where you left off
- **Onboarding** - New team members get up to speed quickly
- **Documentation** - Always up-to-date technical reference

---

## 🔒 Version Control

**Should this folder be committed to git?**

**YES** - Recommended to commit this folder because:
- ✅ Shared context across team members
- ✅ Version controlled with code changes
- ✅ Available on all machines after clone
- ✅ Part of project documentation

**Alternative**: Add to `.gitignore` if context is personal/machine-specific.

---

## 📊 File Sizes

- **PROJECT_CONTEXT.md**: ~450 lines - Comprehensive reference
- **CODEBASE_MAP.md**: ~300 lines - Code navigation
- **DEVELOPMENT_NOTES.md**: ~250 lines - Working notes
- **GIT_COMMIT_GUIDE.md**: ~150 lines - Commit standards
- **CHEATSHEET.md**: ~200 lines - Quick reference
- **README.md**: ~150 lines - Introduction
- **INDEX.md**: ~100 lines - Navigation
- **Total**: ~1,600 lines of context

**Why this size?**
- Fits in most AI context windows
- Detailed enough to be useful
- Concise enough to read quickly
- Structured for easy navigation

---

## 🛠️ Maintenance

### When to Update

**PROJECT_CONTEXT.md**:
- Configuration schema changes
- New major features
- Architecture changes
- Deployment changes

**CODEBASE_MAP.md**:
- New core components
- Interface changes
- Data flow modifications
- New patterns introduced

**DEVELOPMENT_NOTES.md**:
- New development patterns
- Debugging discoveries
- Performance insights
- Design decisions

### How to Update

1. Make code changes
2. Update relevant context file(s)
3. Commit both together
4. Keep context in sync with code

---

## 📚 Related Documentation

This folder complements (not replaces) existing documentation:

- **README.md** - User-facing project introduction
- **docs/API.md** - API endpoint reference
- **docs/QUICKSTART.md** - Getting started guide
- **docs/PROJECT_SUMMARY.md** - High-level summary
- **docs/TROUBLESHOOTING.md** - Common issues

**Difference**: `.augment/` is optimized for AI assistants and developers working with AI tools, while `docs/` is for general users and contributors.

---

## 🤖 AI Assistant Instructions

When you (AI assistant) are asked to work on this project:

1. **Always read PROJECT_CONTEXT.md first** if you haven't in this session
2. **Reference CODEBASE_MAP.md** when navigating code
3. **Follow patterns in DEVELOPMENT_NOTES.md** when making changes
4. **Update these files** if you make significant architectural changes
5. **Suggest updates** to these files if you notice they're outdated

---

## 📞 Contact

If you have questions about this context system or suggestions for improvement, please:
- Open an issue in the repository
- Update these files directly (they're living documents)
- Share feedback with the team

---

**Last Updated**: 2026-01-19
**Maintained By**: Development team + AI assistants
**Purpose**: Enable seamless AI-assisted development across machines and sessions

---

**End of README**

