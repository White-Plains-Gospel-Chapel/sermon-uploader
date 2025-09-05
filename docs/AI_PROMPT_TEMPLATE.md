# 🤖 AI Assistant Prompt Template

> **Purpose**: Copy-paste this into ANY AI chat (ChatGPT, Claude, Gemini, etc.) to ensure consistent documentation.

## 📋 Copy This Entire Block

```
I need you to follow specific documentation standards for this project. Please read and acknowledge these rules:

## Documentation Philosophy
- Create recipe-style guides (copy-paste solutions, not essays)
- Use quick reference cards instead of long explanations
- Be goal-oriented ("I want to..." not theoretical chapters)
- Make everything scannable with emojis and visual hierarchy
- Always include time estimates for tasks

## Structure to Follow

### For HOW-TO Tasks: Use Recipe Format
# [Emoji] Recipe: [Task Name]
> Goal: [What they achieve] | Time: [X min] | Difficulty: Easy/Medium/Hard

- What You Need (checklist)
- Steps (with copy-paste commands)
- Success Check (verification)
- If Something's Wrong (troubleshooting)

### For Technical Docs: Start with TL;DR
# [Emoji] [Title]
> TL;DR: [One sentence - what problem this solves]
- Include time estimates
- Use tables and diagrams
- Provide quick reference section

## Emoji Usage
🚀 Getting Started/Deploy | 🍰 Recipes | ⚡ Commands | 🔧 Config
🐛 Debug | 📊 Architecture | 💻 Development | 🔒 Security
✅ Success | ❌ Error | 🚧 In Progress | 💡 Tips | 🎯 Goals

## Quality Rules
1. TL;DR or goal statement at the top of every doc
2. Time estimates for all tasks
3. Copy-paste ready code blocks
4. Success criteria (how to verify it worked)
5. Troubleshooting section for when things fail
6. Next steps/related links at the end

## Writing Style
- NO walls of text - use headers, lists, tables
- NO starting with theory - start with action/outcome  
- NO jargon without explanation - define or link
- YES to examples in every section
- YES to visual elements (tables, diagrams, emojis)
- YES to "If this breaks, try this" sections

When I ask you to document something:
1. Determine if it's a recipe (task) or technical doc
2. Use the appropriate template above
3. Keep it scannable and action-oriented
4. Include all required sections

Please confirm you understand these documentation standards and will follow them.
```

## 🎯 How to Use This

### With ChatGPT:
1. Start a new conversation
2. Paste the entire block above
3. Wait for acknowledgment
4. Then ask for documentation help

### With Claude:
1. Include at the start of your prompt
2. Or reference: "Follow the documentation standards in `/docs/AI_DOCUMENTATION_GUIDE.md`"

### With GitHub Copilot:
1. Add as a comment in your file
2. Or create `.github/copilot-docs.md` with these rules

### With Other AI:
1. Paste at conversation start
2. Remind if output doesn't follow format
3. Reference specific sections as needed

## 📝 Quick Test

After pasting the template, test with:
```
"Create documentation for setting up Docker on a Raspberry Pi"
```

The AI should respond with:
- A recipe-style guide with emoji headers
- Time estimate at the top
- Copy-paste commands
- Success verification steps
- Troubleshooting section

If not, remind it to follow the template.

## 🔄 Maintaining Consistency

### In Long Conversations
Remind the AI periodically:
```
"Remember to follow the recipe-style documentation format with emojis and time estimates"
```

### When Switching Topics
```
"Continue using the documentation standards we established"
```

### If Output Drifts
```
"Please reformat that as a recipe with: goal, time, steps, success check, and troubleshooting"
```

## 💡 Pro Tips

1. **Save this template** in your notes for reuse
2. **Create a browser bookmark** to this file
3. **Include in project README** for team consistency
4. **Update based on what works** for your project

---

**Remember**: The goal is documentation that developers will actually read and use, not comprehensive manuals they'll ignore!