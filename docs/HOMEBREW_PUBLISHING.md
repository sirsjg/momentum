# Publishing Momentum to Homebrew

This guide explains how to set up Homebrew distribution for Momentum.

## Overview

Momentum uses [GoReleaser](https://goreleaser.com/) to automatically create and publish Homebrew formulas when a new version is tagged and released. The formula is published to a custom Homebrew tap.

## Prerequisites

1. A GitHub account
2. A personal access token (PAT) with `repo` scope
3. The `homebrew-tap` repository created

## Step 1: Create the Homebrew Tap Repository

Create a new repository on GitHub named `homebrew-tap`:

```bash
# On GitHub, create a new repository:
# Repository name: homebrew-tap
# Make it public (required for public taps)
# Initialize with a README
```

Or use the GitHub CLI:

```bash
gh repo create homebrew-tap --public --description "Homebrew formulas for my projects"
```

The repository should follow the naming convention `homebrew-<tap-name>`. Users will tap it as `<username>/tap`.

## Step 2: Create the Formula Directory

In your `homebrew-tap` repository, create the `Formula` directory:

```bash
cd homebrew-tap
mkdir Formula
touch Formula/.gitkeep
git add .
git commit -m "chore: add Formula directory"
git push
```

## Step 3: Create a Personal Access Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Set a descriptive name: `Homebrew Tap Token`
4. Set expiration as needed
5. Select the `repo` scope (full control of private repositories)
6. Click "Generate token"
7. **Copy the token immediately** (you won't see it again)

## Step 4: Add the Token as a Repository Secret

In the `momentum` repository:

1. Go to Settings → Secrets and variables → Actions
2. Click "New repository secret"
3. Name: `HOMEBREW_TAP_GITHUB_TOKEN`
4. Value: Paste the personal access token from Step 3
5. Click "Add secret"

## Step 5: Verify GoReleaser Configuration

The `.goreleaser.yaml` file should have a `brews` section like this:

```yaml
brews:
  - name: momentum
    repository:
      owner: stevegrehan
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/stevegrehan/momentum"
    description: "Headless agent runner for Flux project management"
    license: "MIT"
    install: |
      bin.install "momentum"
    test: |
      system "#{bin}/momentum", "version"
```

**Important**: Update `owner` to match your GitHub username.

## Step 6: Create Your First Release

Once everything is set up, create a release by tagging a version:

```bash
# Using conventional commits triggers semantic versioning
# Example commits:
git commit -m "feat: add new feature"    # Minor version bump
git commit -m "fix: bug fix"              # Patch version bump
git commit -m "feat!: breaking change"    # Major version bump

# Or manually tag a release:
git tag v0.1.0
git push origin v0.1.0
```

The CI pipeline will:
1. Run tests
2. Analyze commits for version bump
3. Create a new tag (if using semantic-release)
4. Trigger GoReleaser
5. Build binaries for all platforms
6. Create a GitHub release
7. Push the Homebrew formula to your tap

## Step 7: Install via Homebrew

Once released, users can install momentum:

```bash
# Add your tap
brew tap stevegrehan/tap

# Install momentum
brew install momentum

# Or in one command
brew install stevegrehan/tap/momentum
```

## Updating Versions

Future releases are automatic:

1. Merge PRs with conventional commit messages to `main`
2. CI runs semantic-release to determine version bump
3. If a release is needed, it creates a tag
4. GoReleaser builds and publishes everything

### Conventional Commit Reference

| Prefix | Version Bump | Example |
|--------|-------------|---------|
| `feat:` | Minor (0.X.0) | `feat: add export command` |
| `fix:` | Patch (0.0.X) | `fix: handle empty response` |
| `perf:` | Patch | `perf: optimize task selection` |
| `feat!:` or `BREAKING CHANGE:` | Major (X.0.0) | `feat!: remove deprecated flag` |
| `docs:` | No release | `docs: update README` |
| `chore:` | No release | `chore: update dependencies` |
| `ci:` | No release | `ci: add linting step` |
| `test:` | No release | `test: add unit tests` |

## Troubleshooting

### Formula not updating

- Check that `HOMEBREW_TAP_GITHUB_TOKEN` is set correctly
- Verify the token has `repo` scope
- Check the GoReleaser logs in the GitHub Actions run

### Users can't find the formula

- Ensure the tap repository is public
- Verify the formula file was pushed to `Formula/momentum.rb`

### Build failures

- Check that tests pass locally: `go test ./...`
- Verify GoReleaser config: `goreleaser check`
- Run a local test build: `goreleaser build --snapshot --clean`

## Manual Formula Creation (Alternative)

If you prefer manual control, create `Formula/momentum.rb` in your tap:

```ruby
class Momentum < Formula
  desc "Headless agent runner for Flux project management"
  homepage "https://github.com/stevegrehan/momentum"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/stevegrehan/momentum/releases/download/v0.1.0/momentum_0.1.0_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    end
    on_intel do
      url "https://github.com/stevegrehan/momentum/releases/download/v0.1.0/momentum_0.1.0_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stevegrehan/momentum/releases/download/v0.1.0/momentum_0.1.0_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    end
    on_intel do
      url "https://github.com/stevegrehan/momentum/releases/download/v0.1.0/momentum_0.1.0_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    end
  end

  def install
    bin.install "momentum"
  end

  test do
    system "#{bin}/momentum", "version"
  end
end
```

## Summary

| Step | Action | Status |
|------|--------|--------|
| 1 | Create `homebrew-tap` repository | Manual |
| 2 | Create `Formula` directory | Manual |
| 3 | Generate Personal Access Token | Manual |
| 4 | Add `HOMEBREW_TAP_GITHUB_TOKEN` secret | Manual |
| 5 | GoReleaser config | Already configured |
| 6 | Create first release | Automatic on conventional commits |
| 7 | Users install via `brew` | Automatic |

After completing steps 1-4, every merged PR with conventional commits will automatically create releases and update the Homebrew formula.
