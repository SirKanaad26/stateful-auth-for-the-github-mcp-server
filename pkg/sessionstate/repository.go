package sessionstate

// RepositoryContext represents the owner and repository name extracted from a tool call.
type RepositoryContext struct {
	Owner string
	Repo  string
}

// ExtractRepositoryFromArgs attempts to extract owner and repo from tool arguments.
// It checks for common parameter names used across GitHub MCP tools.
// Returns nil if no repository context is found.
func ExtractRepositoryFromArgs(args map[string]interface{}) *RepositoryContext {
	// Check for owner parameter
	owner, ownerFound := args["owner"].(string)
	if !ownerFound {
		return nil
	}

	// Check for repo parameter
	repo, repoFound := args["repo"].(string)
	if !repoFound {
		return nil
	}

	// Both owner and repo found
	if owner != "" && repo != "" {
		return &RepositoryContext{
			Owner: owner,
			Repo:  repo,
		}
	}

	return nil
}

// HasRepositoryContext returns true if the repository context can be extracted from the arguments.
func HasRepositoryContext(args map[string]interface{}) bool {
	return ExtractRepositoryFromArgs(args) != nil
}
