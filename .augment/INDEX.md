# .augment Folder - Index

**Quick navigation guide for all context files**

---

## 📚 All Files

### 1. **README.md** ⭐ START HERE
**What**: Introduction to the .augment folder system  
**When**: First time using this folder  
**Size**: ~150 lines  
**Contains**:
- Purpose of this folder
- File descriptions
- Usage examples
- Benefits for AI assistants
- Maintenance guidelines

👉 **Read this first to understand the system**

---

### 2. **PROJECT_CONTEXT.md** 📖 COMPREHENSIVE REFERENCE
**What**: Complete project overview and reference  
**When**: Starting new session, need high-level understanding  
**Size**: ~450 lines  
**Contains**:
- Project overview and purpose
- Architecture (3-layer Clean Architecture)
- Tech stack and dependencies
- Project structure
- Configuration details
- Entry points (server, CLI)
- API endpoints
- Key workflows
- Build & run instructions
- Development guidelines
- Troubleshooting
- Quick reference

👉 **Your main reference document**

---

### 3. **CODEBASE_MAP.md** 🗺️ CODE NAVIGATION
**What**: Detailed code structure and navigation  
**When**: Need to find specific code or understand data flow  
**Size**: ~300 lines  
**Contains**:
- Core domain models (with code snippets)
- Application services (DownloadManager, QueueManager)
- Infrastructure layer (downloaders, repository, notifier)
- API layer (router, handlers, middleware)
- Utilities (logger, validator)
- Data flow examples
- Testing structure
- Configuration loading
- Dependency injection

👉 **Use when navigating the codebase**

---

### 4. **DEVELOPMENT_NOTES.md** 💡 WORKING NOTES
**What**: Development patterns and decisions  
**When**: Making changes, debugging, extending features  
**Size**: ~250 lines  
**Contains**:
- Current implementation state
- Known limitations
- Design decisions and rationale
- Code patterns (error handling, logging)
- Common development tasks (step-by-step guides)
- Testing guidelines
- Debugging tips
- Performance considerations
- Future enhancement ideas

👉 **Use when developing or debugging**

---

### 5. **CHEATSHEET.md** ⚡ QUICK REFERENCE
**What**: Ultra-fast reference for common tasks  
**When**: Need quick command or code snippet  
**Size**: ~200 lines  
**Contains**:
- Quick commands (build, test, docker)
- API quick reference (curl examples)
- CLI quick reference
- File locations
- Configuration quick edit
- Quick debugging
- Key enums
- Code snippets
- Testing patterns

👉 **Use for quick lookups**

---

### 6. **GIT_COMMIT_GUIDE.md** 📝 COMMIT STANDARDS
**What**: Git commit message guidelines
**When**: Writing commit messages
**Size**: ~150 lines
**Contains**:
- Conventional Commits format
- Commit types and scopes
- Rules and best practices
- Good and bad examples
- Common patterns
- Quick reference templates

👉 **Use when committing code**

---

### 7. **INDEX.md** 📑 THIS FILE
**What**: Navigation guide for all context files
**When**: Need to find the right file
**Size**: ~100 lines

👉 **You are here**

---

## 🎯 Quick Decision Tree

**"I'm starting a new session"**
→ Read **PROJECT_CONTEXT.md**

**"I need to find where X is implemented"**
→ Check **CODEBASE_MAP.md**

**"I want to add a new feature"**
→ Read **DEVELOPMENT_NOTES.md** → "Common Development Tasks"

**"I need a quick command"**
→ Check **CHEATSHEET.md**

**"I'm debugging an issue"**
→ Check **DEVELOPMENT_NOTES.md** → "Debugging Tips"

**"I don't understand this folder"**
→ Read **README.md**

**"I need to understand data flow"**
→ Check **CODEBASE_MAP.md** → "Data Flow Examples"

**"What are the design decisions?"**
→ Read **DEVELOPMENT_NOTES.md** → "Design Decisions"

**"How should I write commit messages?"**
→ Read **GIT_COMMIT_GUIDE.md**

---

## 📊 File Comparison

| File | Purpose | Size | Best For |
|------|---------|------|----------|
| README.md | Introduction | 150 lines | Understanding the system |
| PROJECT_CONTEXT.md | Complete reference | 450 lines | High-level overview |
| CODEBASE_MAP.md | Code navigation | 300 lines | Finding code |
| DEVELOPMENT_NOTES.md | Dev patterns | 250 lines | Making changes |
| CHEATSHEET.md | Quick reference | 200 lines | Fast lookups |
| GIT_COMMIT_GUIDE.md | Commit standards | 150 lines | Writing commits |
| INDEX.md | Navigation | 100 lines | Finding right file |

**Total**: ~1,600 lines of comprehensive context

---

## 🔄 Reading Order

### For New AI Assistant Session
1. **PROJECT_CONTEXT.md** - Get the big picture (10 min read)
2. **CODEBASE_MAP.md** - Understand structure (5 min skim)
3. **CHEATSHEET.md** - Bookmark for quick reference

### For Specific Task
1. **CHEATSHEET.md** - Check if quick answer exists
2. **DEVELOPMENT_NOTES.md** - Find step-by-step guide
3. **CODEBASE_MAP.md** - Locate relevant code
4. **PROJECT_CONTEXT.md** - Understand broader context

### For Debugging
1. **DEVELOPMENT_NOTES.md** → "Debugging Tips"
2. **CHEATSHEET.md** → "Quick Debugging"
3. **CODEBASE_MAP.md** → Understand component
4. **PROJECT_CONTEXT.md** → "Troubleshooting"

---

## 🎨 File Icons Legend

- ⭐ = Start here
- 📖 = Comprehensive reference
- 🗺️ = Navigation/map
- 💡 = Tips and patterns
- ⚡ = Quick reference
- 📑 = Index/navigation

---

## 🔍 Search Tips

### Find by Topic

**Architecture**:
- PROJECT_CONTEXT.md → "Architecture"
- CODEBASE_MAP.md → "Core Domain Models"
- DEVELOPMENT_NOTES.md → "Design Decisions"

**API**:
- PROJECT_CONTEXT.md → "API Endpoints"
- CODEBASE_MAP.md → "API Layer"
- CHEATSHEET.md → "API Quick Reference"

**Configuration**:
- PROJECT_CONTEXT.md → "Configuration"
- CODEBASE_MAP.md → "Configuration Loading"
- CHEATSHEET.md → "Configuration Quick Edit"

**Testing**:
- CODEBASE_MAP.md → "Testing Structure"
- DEVELOPMENT_NOTES.md → "Testing Guidelines"
- CHEATSHEET.md → "Testing Patterns"

**Debugging**:
- DEVELOPMENT_NOTES.md → "Debugging Tips"
- CHEATSHEET.md → "Quick Debugging"
- PROJECT_CONTEXT.md → "Troubleshooting"

**Git Commits**:
- GIT_COMMIT_GUIDE.md → Complete guide
- CHEATSHEET.md → "Git Commit Messages"
- DEVELOPMENT_NOTES.md → "Git Commit Messages"

---

## 📝 Maintenance

**When to update each file**:

- **README.md**: When folder structure changes
- **PROJECT_CONTEXT.md**: Major features, config changes, architecture
- **CODEBASE_MAP.md**: New components, interface changes
- **DEVELOPMENT_NOTES.md**: New patterns, debugging discoveries
- **CHEATSHEET.md**: New commands, common tasks
- **GIT_COMMIT_GUIDE.md**: Commit standards change (rarely)
- **INDEX.md**: When new files added to .augment/

---

## 🚀 Getting Started (30 Second Version)

1. Read **README.md** (2 min)
2. Skim **PROJECT_CONTEXT.md** (5 min)
3. Bookmark **CHEATSHEET.md** for quick reference
4. Start coding! 🎉

---

**Last Updated**: 2026-01-27
**Total Context**: ~1,600 lines
**Purpose**: Enable seamless AI-assisted development

---

**End of Index**

