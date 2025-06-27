## GitHub CLI: Fetching PR Comments/Reviews

When you need to see PR review comments, use these specific commands:

### Primary Commands (try these first):
```bash
# Get review comments (inline code comments) - MOST USEFUL
gh pr view {pr_number} --json reviewThreads

# Get general PR reviews and their status
gh pr view {pr_number} --json reviews

# Get both comments and reviews in one call
gh pr view {pr_number} --json reviews,reviewThreads
```

### Alternative Commands (if above don't work):
```bash
# Direct API calls
gh api repos/{owner}/{repo}/pulls/{pr_number}/reviews
gh api repos/{owner}/{repo}/pulls/{pr_number}/comments

# Simple text view (less structured but readable)
gh pr view {pr_number}
```

### Key Points:
- Use `--json reviewThreads` for inline code review comments
- Use `--json reviews` for overall review status/summary
- Review threads contain the actual actionable feedback
- Don't use `gh pr review` - that's for creating reviews, not reading them
