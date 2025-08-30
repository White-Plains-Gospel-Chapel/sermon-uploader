#!/bin/bash
set -e

echo "📋 Setting up GitHub Project for Secure Pi Deployment..."

# Create the project via web interface instructions since CLI has auth limitations
echo ""
echo "🌐 MANUAL PROJECT SETUP:"
echo "Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/projects"
echo ""
echo "Click 'New project' and configure:"
echo "  📝 Project name: 'Secure Pi Deployment Pipeline'"
echo "  📄 Description: 'Implementation tracking for zero-exposure Pi deployment architecture'"
echo "  🔧 Template: 'Feature development' or 'Bug triage'"
echo ""

echo "🏗️ PROJECT STRUCTURE:"
echo "Create these columns/views:"
echo "  📤 Backlog - New issues waiting for assignment"
echo "  🔄 In Progress - Currently being implemented" 
echo "  👀 Review - Ready for @claude-code verification"
echo "  ✅ Done - Approved and merged"
echo ""

echo "🔗 AUTO-LINK ISSUES:"
echo "After creating project:"
echo "  1. Go to project settings"
echo "  2. Enable 'Auto-add to project' for:"
echo "     - New issues with label 'priority:high'"
echo "     - New PRs from this repository"
echo "  3. Set up automation rules:"
echo "     - Move to 'In Progress' when assigned"
echo "     - Move to 'Review' when PR opened"
echo "     - Move to 'Done' when PR merged"
echo ""

# Try to add existing issues to project (if project exists)
echo "📌 ADDING EXISTING ISSUES:"
echo "After project creation, manually add these issues:"

gh issue list --json number,title,labels --template '
{{- range . -}}
  - #{{.number}}: {{.title}}
{{- end -}}'

echo ""
echo "🎯 PROJECT BENEFITS:"
echo "  ✅ Visual kanban board for task tracking"
echo "  ✅ Automated issue/PR workflow"
echo "  ✅ Progress visibility for stakeholders"
echo "  ✅ Milestone tracking integration"
echo "  ✅ Cross-repository project linking"
echo ""

echo "📊 RECOMMENDED VIEWS:"
echo "Create these project views:"
echo "  🔥 Priority View - Grouped by priority labels"
echo "  👤 Assignee View - Grouped by assigned person"  
echo "  🏷️ Status View - Grouped by implementation status"
echo "  📅 Timeline View - Sorted by due dates/milestones"
echo ""

echo "✅ Project setup instructions complete!"
echo "Visit: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/projects"