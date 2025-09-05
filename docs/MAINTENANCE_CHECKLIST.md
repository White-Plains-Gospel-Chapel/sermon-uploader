# 🔧 Documentation Maintenance Checklist

> **Purpose**: Weekly/monthly checklist to keep documentation fresh and organized

## 📅 Weekly Tasks (Fridays, 15 minutes)

### 🔍 Check for Drift
- [ ] Any new features without recipes?
- [ ] Any new errors without troubleshooting docs?
- [ ] Any new commands not in quick reference?
- [ ] Any completed work still in `planning/active/`?

### 📝 Quick Updates
- [ ] Update CHANGELOG with this week's changes
- [ ] Move completed planning docs to `planning/completed/`
- [ ] Check if START_HERE still accurate for beginners
- [ ] Verify all recipe commands still work

### 🧹 Cleanup
- [ ] Remove duplicate documentation
- [ ] Fix any broken internal links
- [ ] Update time estimates if they're wrong
- [ ] Archive outdated troubleshooting

## 📅 Monthly Tasks (First Monday, 30 minutes)

### 📊 Organization Review
- [ ] All new files in correct folders?
- [ ] Version number updated in `.version`?
- [ ] Release notes created for new version?
- [ ] README indexes updated with new docs?

### 🎯 Quality Check
- [ ] All docs have TL;DR or goal statement?
- [ ] All recipes have time estimates?
- [ ] All technical docs have visual diagrams?
- [ ] All troubleshooting has solutions?

### 🔄 Consistency Audit
- [ ] Emojis used consistently?
- [ ] Templates being followed?
- [ ] Navigation guide still accurate?
- [ ] Quick reference cards complete?

### 📈 Usage Analysis
- [ ] What questions keep coming up? (need recipes)
- [ ] What docs are never referenced? (consider removing)
- [ ] What tasks take longer than estimated? (update times)
- [ ] What's missing based on user feedback?

## 🚀 After Major Changes

### New Feature Checklist
- [ ] Created recipe for using feature
- [ ] Updated architecture docs if needed
- [ ] Added to NAVIGATION.md
- [ ] Updated START_HERE if affects beginners
- [ ] Added new commands to quick reference
- [ ] Created troubleshooting section
- [ ] Updated CHANGELOG
- [ ] Created release notes if new version

### After Refactoring
- [ ] Updated all affected recipes
- [ ] Moved planning docs to completed
- [ ] Updated architecture diagrams
- [ ] Added migration guide if breaking
- [ ] Updated quick reference commands
- [ ] Tested all documentation examples

### After Bug Fixes
- [ ] Added to troubleshooting
- [ ] Updated relevant recipes
- [ ] Added to known issues if not fully fixed
- [ ] Updated quick reference if workaround needed

## 🔍 Documentation Smells

### Signs Documentation Needs Work

#### 🚨 Red Flags (Fix Immediately)
- [ ] Users asking questions answered in docs
- [ ] Copy-paste commands don't work
- [ ] Wrong version numbers
- [ ] Broken links
- [ ] Missing critical features

#### ⚠️ Yellow Flags (Fix This Week)
- [ ] No emoji headers (hard to scan)
- [ ] No time estimates
- [ ] Walls of text
- [ ] No troubleshooting sections
- [ ] Outdated screenshots

#### 📝 Improvement Opportunities
- [ ] Could use more diagrams
- [ ] Could add more recipes
- [ ] Could improve search keywords
- [ ] Could add more examples
- [ ] Could simplify language

## 📋 Quick Audit Commands

### Find Docs Without Emojis
```bash
find docs -name "*.md" -exec grep -L "^#.*[🚀🍰⚡🔧🐛📊💻🔒✅❌🚧💡🎯]" {} \;
```

### Find Docs Without TL;DR
```bash
find docs -name "*.md" -exec grep -L "TL;DR\|Goal\|Purpose" {} \;
```

### Find Long Documents (might need breaking up)
```bash
find docs -name "*.md" -exec wc -l {} \; | sort -rn | head -10
```

### Find Stale Documents (not updated in 60 days)
```bash
find docs -name "*.md" -mtime +60 -exec ls -la {} \;
```

### Check for Broken Internal Links
```bash
for file in $(find docs -name "*.md"); do
  grep -o '\[.*\]([^)]*\.md)' "$file" | while read link; do
    path=$(echo "$link" | grep -o '([^)]*)' | tr -d '()')
    if [[ ! -f "docs/$path" && ! -f "$path" ]]; then
      echo "Broken link in $file: $path"
    fi
  done
done
```

## 🎯 Success Metrics

### Documentation is Healthy When:
- ✅ New users get running in <10 minutes
- ✅ Developers find answers without asking
- ✅ Commands can be copy-pasted directly
- ✅ Troubleshooting actually solves problems
- ✅ Time estimates are accurate ±20%
- ✅ Visual hierarchy makes scanning easy
- ✅ Updates happen within a week of changes

### Documentation Needs Help When:
- ❌ Same questions asked repeatedly
- ❌ Users say "docs are out of date"
- ❌ Copy-paste commands fail
- ❌ Can't find information quickly
- ❌ No docs for new features
- ❌ Troubleshooting doesn't help
- ❌ Wall of text syndrome

## 🔄 Maintenance Rotation

### If Working in a Team

| Week | Person | Focus Area |
|------|--------|------------|
| 1 | Dev A | Recipes & Quick Start |
| 2 | Dev B | Architecture & Technical |
| 3 | Dev C | Troubleshooting & Operations |
| 4 | Dev D | Planning & Releases |

### Solo Maintenance

| Week | Focus |
|------|--------|
| 1 | User-facing docs (recipes, quick start) |
| 2 | Technical docs (architecture, development) |
| 3 | Operational docs (troubleshooting, deployment) |
| 4 | Planning and releases |

## 💡 Pro Tips

1. **Set a calendar reminder** for weekly/monthly checks
2. **Fix immediately** when you notice issues
3. **Ask users** what's confusing
4. **Test your own docs** - can you follow them?
5. **Keep this checklist updated** as you learn

---

**Remember**: Good documentation is a living system, not a one-time task. Regular small updates prevent documentation debt!