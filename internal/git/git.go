package git

const (
	ProviderUnknown = "unknown"
	ProviderGitHub  = "github"
	ProviderGitLab  = "gitlab"
)

type RemoteInfo struct {
	Provider      string
	Host          string
	NamespacePath string
}

// ParseRemoteBase attempts to infer git provider/host/namespace from profile.gitServerUrl and profile.cpiPath.
// It is best-effort and primarily supports GitHub/GitLab (incl. SSH remotes like git@host:group/subgroup).
func ParseRemoteBase(gitServerURL, cpiPath string) (RemoteInfo, error) {
	return parseRemoteBase(gitServerURL, cpiPath)
}

func NewProvider(provider string) Provider {
	switch provider {
	case ProviderGitHub:
		return &githubProvider{}
	case ProviderGitLab:
		return &gitlabProvider{}
	default:
		return nil
	}
}
