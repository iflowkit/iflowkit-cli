package sync

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iflowkit/iflowkit-cli/internal/app"
	"github.com/iflowkit/iflowkit-cli/internal/common/cpix"
	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/common/gitx"
	"github.com/iflowkit/iflowkit-cli/internal/git"
	"github.com/iflowkit/iflowkit-cli/internal/logging"
	"github.com/iflowkit/iflowkit-cli/internal/models"
)

func runSyncInit(ctx *app.Context, args []string) (err error) {
	defer func() {
		if err != nil {
			ctx.Logger.Error("sync init failed", logging.F("error", err.Error()))
		}
	}()

	fs := flag.NewFlagSet("iflowkit sync init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var packageID string
	var dir string
	fs.StringVar(&packageID, "id", "", "CPI IntegrationPackage id (e.g. com.iflowkit.cpi.email)")
	fs.StringVar(&dir, "dir", "", "Parent directory where <packageId>/ will be created (default: current directory)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		syncInitHelp(ctx)
		return err
	}
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		syncInitHelp(ctx)
		return fmt.Errorf("--id is required")
	}
	if err := validatePackageID(packageID); err != nil {
		return err
	}

	profileID, source, err := ctx.Stores.ResolveProfileID(ctx.Flags.ProfileID)
	if err != nil {
		return err
	}
	prof, err := ctx.Stores.Profiles.Read(profileID)
	if err != nil {
		return err
	}
	ctx.Logger.Info("resolved profile", logging.F("profile", profileID), logging.F("source", source))

	// DEV only.
	tenant, err := ctx.Stores.Tenants.Read(profileID, "dev")
	if err != nil {
		return fmt.Errorf("DEV tenant not found for profile %q; import it with `iflowkit tenant import --env dev --file <service-key.json>`: %w", profileID, err)
	}

	remote, err := git.BuildRemoteURL(prof.GitServerURL, prof.CPIPath, packageID)
	if err != nil {
		return err
	}
	providerName := git.DetectProviderFromRemote(remote)
	ctx.Logger.Info("git remote resolved", logging.F("remote", remote), logging.F("provider", providerName))

	// Fetch package name (required).
	c := cpix.NewClient(tenant, ctx.Logger)
	pkg, raw, err := c.ReadIntegrationPackage(packageID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(pkg.Name) == "" {
		return fmt.Errorf("CPI IntegrationPackage Name is empty (packageId=%s)", packageID)
	}

	// Determine target directory.
	cwd, _ := os.Getwd()
	var absDir string
	if strings.TrimSpace(dir) == "" {
		absDir, err = filepath.Abs(filepath.Join(cwd, packageID))
		if err != nil {
			return err
		}
	} else {
		parentAbs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		st, err := os.Stat(parentAbs)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("--dir path does not exist: %s", parentAbs)
			}
			return err
		}
		if !st.IsDir() {
			return fmt.Errorf("--dir is not a directory: %s", parentAbs)
		}
		absDir = filepath.Join(parentAbs, packageID)
	}
	if err := ensureEmptyDir(absDir); err != nil {
		return err
	}

	ctx.Logger.Info("sync init started", logging.F("packageId", packageID), logging.F("packageName", pkg.Name))

	// Create remote repo (GitHub/GitLab). Unknown providers: best-effort push only.
	ns, repoPath, err := git.SplitRemoteNamespaceAndRepo(remote)
	if err != nil {
		return err
	}
	host, err := git.RemoteHost(remote)
	if err != nil {
		return err
	}

	provider := git.NewProvider(providerName)
	if provider != nil {
		token, terr := git.ResolveToken(providerName)
		if terr != nil {
			return terr
		}
		displayName := provider.NormalizeRepoDisplayName(pkg.Name)
		ctx.Logger.Info("creating git repository", logging.F("provider", providerName), logging.F("namespace", ns), logging.F("repo", repoPath), logging.F("displayName", displayName), logging.F("private", true))
		if err := provider.CreateRepo(context.Background(), token, host, ns, repoPath, displayName, true); err != nil {
			return err
		}
		ctx.Logger.Info("git repository ready", logging.F("remote", remote))
	} else {
		ctx.Logger.Warn("git provider not supported for automatic repo creation; will attempt push", logging.F("provider", providerName), logging.F("remote", remote))
	}

	// Export CPI artifacts into the repository structure.
	baseFolder := filepath.Join(absDir, "IntegrationPackage")
	if err := c.ExportIntegrationPackageFromRaw(packageID, raw, baseFolder); err != nil {
		return err
	}

	// Write sync metadata.
	meta := models.SyncMetadata{
		SchemaVersion:   1,
		ProfileID:       prof.ID,
		CPITenantLevels: prof.CPITenantLevels,
		PackageID:       packageID,
		PackageName:     pkg.Name,
		BaseFolder:      "IntegrationPackage",
		GitRemote:       remote,
		GitProvider:     providerName,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := meta.ValidateRequired(); err != nil {
		return err
	}
	b, err := meta.PrettyJSON()
	if err != nil {
		return err
	}
	if err := filex.EnsureDir(filepath.Join(absDir, ".iflowkit")); err != nil {
		return err
	}
	if err := filex.AtomicWriteFile(filepath.Join(absDir, ".iflowkit", "package.json"), b, 0o644); err != nil {
		return err
	}
	if err := EnsureRepoIgnoreFile(absDir); err != nil {
		return err
	}
	if err := ensureSyncRepoGitignore(absDir); err != nil {
		return err
	}

	// Create init transport record + update index.json.
	transportID, createdAt := newTransportIDs(time.Now())
	if err := writeInitTransport(ctx, absDir, meta.BaseFolder, meta.PackageID, "dev", "dev", transportID, createdAt); err != nil {
		return err
	}

	if err := initGitRepo(ctx, absDir, remote, transportID); err != nil {
		return err
	}

	ctx.Logger.Info("sync init completed", logging.F("dir", absDir), logging.F("remote", remote), logging.F("branch", "dev"))
	fmt.Fprintf(ctx.Stdout, "Initialized sync repo for %s (%s)\nRemote: %s\nBranch: dev\nDirectory: %s\n", packageID, pkg.Name, remote, absDir)
	return nil
}

func ensureSyncRepoGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	defaultLines := []string{".DS_Store", "*.log"}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			content := strings.Join(defaultLines, "\n") + "\n"
			return filex.AtomicWriteFile(path, []byte(content), 0o644)
		}
		return err
	}

	content := string(b)
	for _, line := range defaultLines {
		if strings.Contains(content, line) {
			continue
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += line + "\n"
	}
	return filex.AtomicWriteFile(path, []byte(content), 0o644)
}

func ensureEmptyDir(dir string) error {
	st, err := os.Stat(dir)
	if err == nil {
		if !st.IsDir() {
			return fmt.Errorf("target path exists and is not a directory: %s", dir)
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(ents) > 0 {
			return fmt.Errorf("target directory is not empty: %s", dir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func initGitRepo(ctx *app.Context, dir string, remote string, transportID string) error {
	// Ensure git exists.
	if err := gitx.LookPath(); err != nil {
		return err
	}

	// git init
	if err := runGit(ctx, dir, "init"); err != nil {
		return err
	}
	// configure identity to avoid commit failures in clean environments
	_ = runGit(ctx, dir, "config", "user.email", "iflowkit@local")
	_ = runGit(ctx, dir, "config", "user.name", "iFlowKit CLI")

	// dev branch
	if err := runGit(ctx, dir, "checkout", "-b", "dev"); err != nil {
		return err
	}

	// 1) contents commit (IntegrationPackage only)
	if err := runGit(ctx, dir, "add", "-A", "--", "IntegrationPackage"); err != nil {
		return err
	}
	msg1 := buildTransportCommitMessage(transportID, "init", "contents", "")
	if err := runGit(ctx, dir, "commit", "-m", msg1, "--", "IntegrationPackage"); err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") {
			return err
		}
	}

	// 2) logs commit (everything outside IntegrationPackage, including .iflowkit)
	if err := runGit(ctx, dir, "add", "-A"); err != nil {
		return err
	}
	_ = runGit(ctx, dir, "reset", "HEAD", "--", "IntegrationPackage")
	_ = runGit(ctx, dir, "add", "-f", "--", ".iflowkit")
	out, _ := runGitOutput(ctx, dir, "diff", "--cached", "--name-only")
	if strings.TrimSpace(out) != "" {
		msg2 := buildTransportCommitMessage(transportID, "init", "logs", "")
		if err := runGit(ctx, dir, "commit", "-m", msg2); err != nil {
			if !strings.Contains(err.Error(), "nothing to commit") {
				return err
			}
		}
	}

	if err := runGit(ctx, dir, "remote", "add", "origin", remote); err != nil {
		// ignore if remote exists
		if !strings.Contains(err.Error(), "remote origin already exists") {
			return err
		}
	}

	if err := runGit(ctx, dir, "push", "-u", "origin", "dev"); err != nil {
		return err
	}

	return nil
}

func validatePackageID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("packageId is required")
	}
	if strings.ContainsAny(id, " \t\r\n") {
		return fmt.Errorf("packageId must not contain whitespace")
	}
	if strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("packageId must not contain path separators")
	}
	if len(id) > 128 {
		return fmt.Errorf("packageId is too long (max 128 characters)")
	}
	return nil
}
