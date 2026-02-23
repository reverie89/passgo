// multipass.go - Functions to interact with Multipass command-line tool
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runMultipassCommand executes multipass commands with variadic arguments
func runMultipassCommand(args ...string) (string, error) {
	cmd := exec.Command("multipass", args...) // #nosec G204 -- multipass CLI wrapper
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if appLogger != nil {
		appLogger.Printf("exec: multipass %s", strings.Join(args, " "))
	}
	err := cmd.Run()
	if err != nil {
		if appLogger != nil {
			appLogger.Printf("exec error: %v; stderr: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("command failed: %v\nStderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// NetworkInfo represents an interface from multipass networks.
// Works on Linux (QEMU), Windows (Hyper-V/VirtualBox), macOS (QEMU/VirtualBox).
type NetworkInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// networksJSON is the response structure from multipass networks --format json.
type networksJSON struct {
	List []NetworkInfo `json:"list"`
}

// ListNetworks returns available interfaces for bridged networking.
// Returns nil slice and error if multipass networks is unsupported (e.g. Linux LXD).
func ListNetworks() ([]NetworkInfo, error) {
	output, err := runMultipassCommand("networks", "--format", "json")
	if err != nil {
		if appLogger != nil {
			appLogger.Printf("multipass networks unavailable: %v", err)
		}
		return nil, err
	}
	var resp networksJSON
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		if appLogger != nil {
			appLogger.Printf("multipass networks parse error: %v", err)
		}
		return nil, fmt.Errorf("failed to parse networks: %w", err)
	}
	return resp.List, nil
}

// LaunchVM creates a new virtual machine with basic settings
func LaunchVM(name, release string) (string, error) {
	args := []string{"launch", "--name", name, release}
	return runMultipassCommand(args...)
}

// LaunchVMAdvanced creates VM with custom resource settings.
// networkName: "" = NAT, "bridged" = --bridged (uses configured default), else --network <name>.
func LaunchVMAdvanced(name, release string, cpus int, memoryMB int, diskGB int, networkName string) (string, error) {
	args := []string{
		"launch",
		"--name", name,
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%dM", memoryMB),
		"--disk", fmt.Sprintf("%dG", diskGB),
	}
	if networkName == "bridged" {
		args = append(args, "--bridged")
	} else if networkName != "" {
		args = append(args, "--network", networkName)
	}
	args = append(args, release)
	return runMultipassCommand(args...)
}

func ListVMs() (string, error) {
	return runMultipassCommand("list")
}

func StopVM(name string) (string, error) {
	return runMultipassCommand("stop", name)
}

func StartVM(name string) (string, error) {
	return runMultipassCommand("start", name)
}

func DeleteVM(name string, purge bool) (string, error) {
	args := []string{"delete", name}
	if purge {
		args = append(args, "--purge")
	}
	return runMultipassCommand(args...)
}

func RecoverVM(name string) (string, error) {
	return runMultipassCommand("recover", name)
}

func ExecInVM(vmName string, commandArgs ...string) (string, error) {
	args := append([]string{"exec", vmName, "--"}, commandArgs...)
	return runMultipassCommand(args...)
}

func ShellVM(vmName string) error {
	cmd := exec.Command("multipass", "shell", vmName) // #nosec G204 -- VM name from user selection
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func GetVMInfo(name string) (string, error) {
	return runMultipassCommand("info", name)
}

func CreateSnapshot(vmName, snapshotName, description string) (string, error) {
	args := []string{"snapshot", "--name", snapshotName, "--comment", description, vmName}
	return runMultipassCommand(args...)
}

func ListSnapshots() (string, error) {
	return runMultipassCommand("list", "--snapshots")
}

func RestoreSnapshot(vmName, snapshotName string) (string, error) {
	snapshotID := vmName + "." + snapshotName
	args := []string{"restore", "--destructive", snapshotID}
	return runMultipassCommand(args...)
}

func DeleteSnapshot(vmName, snapshotName string) (string, error) {
	snapshotID := vmName + "." + snapshotName
	args := []string{"delete", "--purge", snapshotID}
	return runMultipassCommand(args...)
}

// ScanCloudInitFiles finds YAML files with "#cloud-config" header for VM configuration
func ScanCloudInitFiles() ([]string, error) {
	options, err := scanCloudInitTemplateOptions(appSearchDirs())
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(options))
	for _, opt := range options {
		files = append(files, opt.Label)
	}
	return files, nil
}

// LaunchVMWithCloudInit creates VM with cloud-init.
// networkName: "" = NAT, "bridged" = --bridged, else --network <name>.
func LaunchVMWithCloudInit(name, release string, cpus int, memoryMB int, diskGB int, cloudInitFile, networkName string) (string, error) {
	args := []string{
		"launch",
		"--name", name,
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%dM", memoryMB),
		"--disk", fmt.Sprintf("%dG", diskGB),
		"--cloud-init", cloudInitFile,
	}
	if networkName == "bridged" {
		args = append(args, "--bridged")
	} else if networkName != "" {
		args = append(args, "--network", networkName)
	}
	args = append(args, release)
	return runMultipassCommand(args...)
}

// TemplateOption represents a selectable cloud-init template
type TemplateOption struct {
	Label string
	Path  string
}

var errConfigRepoNotFound = errors.New("github-cloud-init-repo not found")

func appSearchDirs() []string {
	exePath, _ := os.Executable()
	cwd, _ := os.Getwd()
	return appSearchDirsFrom(exePath, cwd)
}

func appSearchDirsFrom(exePath, cwd string) []string {
	var dirs []string

	if exePath != "" {
		dirs = append(dirs, filepath.Dir(exePath))
	}
	if cwd != "" {
		dirs = append(dirs, cwd)
	}

	seen := make(map[string]struct{}, len(dirs))
	var deduped []string
	for _, dir := range dirs {
		normalized := normalizePath(dir)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		deduped = append(deduped, normalized)
	}
	return deduped
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if abs, err := filepath.Abs(clean); err == nil {
		return abs
	}
	return clean
}

func isYAMLFileName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")
}

func hasCloudConfigHeader(filePath string) bool {
	fileHandle, err := os.Open(filePath) // #nosec G304 -- discovered from app search dirs
	if err != nil {
		return false
	}
	defer fileHandle.Close()

	scanner := bufio.NewScanner(fileHandle)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()) == "#cloud-config"
	}
	return false
}

func scanCloudInitTemplateOptions(searchDirs []string) ([]TemplateOption, error) {
	seenPaths := make(map[string]struct{})
	seenLabels := make(map[string]string)
	var options []TemplateOption

	var firstErr error
	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to read directory %s: %w", dir, err)
			}
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fileName := entry.Name()
			if !isYAMLFileName(fileName) {
				continue
			}

			filePath := normalizePath(filepath.Join(dir, fileName))
			if filePath == "" || !hasCloudConfigHeader(filePath) {
				continue
			}
			if _, exists := seenPaths[filePath]; exists {
				continue
			}

			label := fileName
			if existingPath, exists := seenLabels[label]; exists && existingPath != filePath {
				candidate := fmt.Sprintf("%s (%s)", fileName, filepath.Base(dir))
				if _, conflict := seenLabels[candidate]; conflict {
					candidate = fmt.Sprintf("%s (%s)", fileName, dir)
				}
				label = candidate
			}

			seenPaths[filePath] = struct{}{}
			seenLabels[label] = filePath
			options = append(options, TemplateOption{Label: label, Path: filePath})
		}
	}

	if len(options) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return options, nil
}

func readConfigGithubRepoFromFile(configPath string) (string, error) {
	file, err := os.Open(configPath) // #nosec G304 -- path from app search dirs
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	key := "github-cloud-init-repo"
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, key) {
			continue
		}
		idx := strings.Index(line, key)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len(key):])
		if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, ":") {
			rest = strings.TrimSpace(rest[1:])
		}
		rest = strings.TrimLeft(rest, " \t")
		rest = strings.TrimPrefix(rest, "@")
		if rest != "" {
			return rest, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errConfigRepoNotFound
}

func readConfigGithubRepoFromDirs(searchDirs []string) (string, error) {
	var firstErr error

	for _, dir := range searchDirs {
		configPath := filepath.Join(dir, ".config")
		if appLogger != nil {
			appLogger.Printf("reading config: %s", configPath)
		}

		repoURL, err := readConfigGithubRepoFromFile(configPath)
		if err == nil {
			if appLogger != nil {
				appLogger.Printf("config repo url: %s", repoURL)
			}
			return repoURL, nil
		}
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errConfigRepoNotFound) {
			continue
		}

		if firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		return "", firstErr
	}
	return "", fmt.Errorf("github-cloud-init-repo not found in .config")
}

// ReadConfigGithubRepo reads .config from preferred app search directories.
func ReadConfigGithubRepo() (string, error) {
	return readConfigGithubRepoFromDirs(appSearchDirs())
}

// CloneRepoAndScanYAMLs clones the provided repo into a temp dir and returns cloud-init YAML templates found
func CloneRepoAndScanYAMLs(repoURL string) ([]TemplateOption, string, error) {
	if repoURL == "" {
		return nil, "", fmt.Errorf("empty repo URL")
	}

	tmpDir, err := os.MkdirTemp("", "passgo-cloudinit-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	if appLogger != nil {
		appLogger.Printf("cloning repo %s into %s", repoURL, tmpDir)
	}

	// Shallow clone
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir) // #nosec G204 -- repo URL from user .config
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if appLogger != nil {
			appLogger.Printf("git clone failed: %v; %s", err, strings.TrimSpace(stderr.String()))
		}
		_ = os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("git clone failed: %v; %s", err, stderr.String())
	}

	// Walk repo and collect all .yml/.yaml files (no header requirement)
	var options []TemplateOption
	err = filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
			return nil
		}
		rel, _ := filepath.Rel(tmpDir, path)
		label := "repo/" + rel
		options = append(options, TemplateOption{Label: label, Path: path})
		return nil
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("failed to scan repo: %v", err)
	}
	if appLogger != nil {
		appLogger.Printf("found %d yaml templates in repo", len(options))
	}

	return options, tmpDir, nil
}

// GetAllCloudInitTemplateOptions aggregates local and (optional) repo templates.
// Returns the options, any temp dirs to cleanup after use, and error.
func GetAllCloudInitTemplateOptions() ([]TemplateOption, []string, error) {
	var all []TemplateOption
	var cleanupDirs []string

	// Local templates (preferred search dirs)
	local, err := scanCloudInitTemplateOptions(appSearchDirs())
	if err == nil {
		all = append(all, local...)
		if appLogger != nil {
			appLogger.Printf("found %d local cloud-init templates", len(local))
		}
	}

	// Repo templates via .config
	if repoURL, err := ReadConfigGithubRepo(); err == nil && repoURL != "" {
		if opts, tmpDir, err := CloneRepoAndScanYAMLs(repoURL); err == nil {
			all = append(all, opts...)
			if tmpDir != "" {
				cleanupDirs = append(cleanupDirs, tmpDir)
			}
			if appLogger != nil {
				appLogger.Printf("aggregated %d total templates (local+repo)", len(all))
			}
		} else if appLogger != nil {
			appLogger.Printf("repo scan error: %v", err)
		}
	}

	return all, cleanupDirs, nil
}

// CleanupTempDirs removes temporary directories created during repo cloning
func CleanupTempDirs(dirs []string) {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if appLogger != nil {
			appLogger.Printf("cleanup temp dir: %s", d)
		}
		_ = os.RemoveAll(d)
	}
}
