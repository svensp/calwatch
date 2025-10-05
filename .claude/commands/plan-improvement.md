# Plan New Improvement

Create a new improvement planning document for CalWatch following the established pattern.

## What this command does

This command will:
1. Analyze the current state of CalWatch and existing improvements
2. Suggest a specific improvement topic based on codebase analysis
3. Ask you what the improvement should actually contain
4. Create a new improvement document (docs/improvement-XXX.md) based on your input
5. Guide you through planning the improvement
6. Follow the improvement documentation standards from CLAUDE.md

## Instructions

When you run this command, I will:

1. **Check existing improvements**: Review docs/improvement-001.md, docs/improvement-002.md, etc. to understand what's been implemented
2. **Analyze current codebase**: Understand the current architecture and identify potential areas for enhancement
3. **Suggest a topic**: Based on my analysis, suggest one specific improvement area such as:
   - Web interface/HTTP API for remote control
   - Multiple notification backends (email, Slack, mobile push)
   - Advanced calendar features (conflict detection, analytics)
   - Configuration management interface
   - Performance and scalability enhancements
   - Enhanced error handling and diagnostics
4. **Ask for your input**: Request that you describe what the improvement should actually contain and implement
5. **Create planning document**: Generate docs/improvement-XXX.md (with next sequential number) based on your requirements
6. **Enter planning mode**: Follow CLAUDE.md requirements - only edit the improvement file and CLAUDE.md during planning

## Planning Mode Requirements

As per CLAUDE.md, during planning I will:
- **NOT edit any files** except CLAUDE.md and the improvement document
- **NOT run any tools** that modify the system state
- **Only research and plan** - no implementation until you explicitly approve the plan
- **Wait for your approval** before proceeding with any implementation

## Usage

Simply say "plan a new improvement" or "start planning improvement 004" and I'll begin the planning process following this workflow.

## Output

You'll get:
- A new docs/improvement-XXX.md file with comprehensive planning
- Analysis of potential enhancement areas
- Detailed implementation plan with phases and technical details
- Clear success criteria and file modification lists
- Ready-to-approve plan for implementation

This follows the established CalWatch improvement process and maintains the high-quality documentation standards.