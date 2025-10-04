# Improvement 001: Documentation Reorganization

## Context

User requested to move design.md and progress.md into a docs/ folder for better organization as the project continues to grow with more documentation files.

## Conversation Summary

**User Request**: "Please move the design.md and progress.md into a docs/ folder because we'll continue to work, creating more files like them"

**Implementation**:
1. Created `docs/` directory
2. Moved `design.md` and `progress.md` to `docs/` folder
3. Updated all references to these files throughout the codebase:
   - **README.md**: Updated 3 references to use `docs/design.md` and `docs/progress.md`
   - **default.nix**: Updated to copy the docs/ directory instead of individual files
   - **calwatch.service**: Updated documentation path to `docs/design.md`

## Files Modified

- `README.md` - Updated documentation links
- `default.nix` - Updated installation script to copy docs/ directory
- `calwatch.service` - Updated documentation reference
- `design.md` → `docs/design.md` (moved)
- `progress.md` → `docs/progress.md` (moved)

## Git Commit

Commit `dc73b8e`: "Reorganize documentation into docs/ folder"

## Outcome

Documentation is now properly organized in a dedicated docs/ folder, providing a clean structure for future documentation additions.

## Next Steps

User indicated this is the first improvement and wants to establish a pattern for improvement documentation files.