# Commit and Push

Commit and push changes to git without mentioning Claude Code in commit messages.

## Steps

1. Run `git status` in parallel with `git log --oneline -10` to see current changes and recent commit style
2. Run `git diff --stat` to see what changed
3. Reset staging and add all changes: `git reset && git add -A`
4. Draft a concise commit message (1-2 sentences) that:
   - Follows the style of recent commits
   - Focuses on "why" rather than "what"
   - Does NOT mention Claude Code or AI
   - Uses lowercase, imperative mood
5. Commit and push using:
   ```bash
   git commit -m "$(cat <<'EOF'
   [commit message here]
   EOF
   )" && git push
   ```
6. Show the commit hash and confirm push was successful
