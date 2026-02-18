package team

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArtifactType represents the type of artifact.
type ArtifactType string

const (
	ArtifactCode     ArtifactType = "code"
	ArtifactDocument ArtifactType = "document"
	ArtifactConfig   ArtifactType = "config"
	ArtifactTest     ArtifactType = "test"
	ArtifactOther    ArtifactType = "other"
)

// Artifact represents a work product created by the team.
type Artifact struct {
	Name        string
	Type        ArtifactType
	Content     string
	Description string
	CreatedBy   string
	CreatedAt   time.Time
	Path        string // Set after saving
}

// NewArtifact creates a new artifact.
func NewArtifact(name string, artifactType ArtifactType, content, description, createdBy string) Artifact {
	return Artifact{
		Name:        name,
		Type:        artifactType,
		Content:     content,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}
}

// Save writes the artifact to the specified directory.
func (a *Artifact) Save(dir string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Sanitize filename to prevent path traversal
	safeName := filepath.Base(filepath.Clean(a.Name))
	if safeName == "." || safeName == ".." || safeName == "" {
		return fmt.Errorf("invalid artifact name: %s", a.Name)
	}

	path := filepath.Join(dir, safeName)
	
	// Verify the final path is within the target directory
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory path: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving file path: %w", err)
	}
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		return fmt.Errorf("path traversal detected: artifact path outside target directory")
	}

	if err := os.WriteFile(path, []byte(a.Content), 0640); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	a.Path = path
	return nil
}

// GetExtension returns the appropriate file extension for the artifact type.
func (a *Artifact) GetExtension() string {
	switch a.Type {
	case ArtifactCode:
		// Try to detect from name
		if ext := filepath.Ext(a.Name); ext != "" {
			return ext
		}
		return ".txt"
	case ArtifactDocument:
		return ".md"
	case ArtifactConfig:
		return ".yaml"
	case ArtifactTest:
		return "_test.go"
	default:
		return ".txt"
	}
}

// ArtifactCollection manages a collection of artifacts.
type ArtifactCollection struct {
	artifacts []Artifact
	baseDir   string
}

// NewArtifactCollection creates a new artifact collection.
func NewArtifactCollection(baseDir string) *ArtifactCollection {
	return &ArtifactCollection{
		artifacts: []Artifact{},
		baseDir:   baseDir,
	}
}

// Add adds an artifact to the collection.
func (c *ArtifactCollection) Add(artifact Artifact) {
	c.artifacts = append(c.artifacts, artifact)
}

// SaveAll saves all artifacts in the collection.
func (c *ArtifactCollection) SaveAll() error {
	if c.baseDir == "" {
		return fmt.Errorf("no base directory set")
	}

	for i := range c.artifacts {
		if err := c.artifacts[i].Save(c.baseDir); err != nil {
			return fmt.Errorf("saving %s: %w", c.artifacts[i].Name, err)
		}
	}

	return nil
}

// List returns all artifacts.
func (c *ArtifactCollection) List() []Artifact {
	return c.artifacts
}

// GetByType returns artifacts of a specific type.
func (c *ArtifactCollection) GetByType(t ArtifactType) []Artifact {
	var result []Artifact
	for _, a := range c.artifacts {
		if a.Type == t {
			result = append(result, a)
		}
	}
	return result
}

// Summary returns a summary of the collection.
func (c *ArtifactCollection) Summary() string {
	if len(c.artifacts) == 0 {
		return "No artifacts"
	}

	summary := fmt.Sprintf("%d artifacts:\n", len(c.artifacts))
	for _, a := range c.artifacts {
		saved := ""
		if a.Path != "" {
			saved = fmt.Sprintf(" -> %s", a.Path)
		}
		summary += fmt.Sprintf("  - %s (%s)%s\n", a.Name, a.Type, saved)
	}
	return summary
}

// GenerateManifest creates a manifest file listing all artifacts.
func (c *ArtifactCollection) GenerateManifest() Artifact {
	var content string
	content = "# Artifact Manifest\n\n"
	content += fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339))
	content += "## Artifacts\n\n"

	for _, a := range c.artifacts {
		content += fmt.Sprintf("### %s\n", a.Name)
		content += fmt.Sprintf("- **Type:** %s\n", a.Type)
		content += fmt.Sprintf("- **Description:** %s\n", a.Description)
		content += fmt.Sprintf("- **Created By:** %s\n", a.CreatedBy)
		content += fmt.Sprintf("- **Created At:** %s\n", a.CreatedAt.Format(time.RFC3339))
		if a.Path != "" {
			content += fmt.Sprintf("- **Path:** %s\n", a.Path)
		}
		content += "\n"
	}

	return NewArtifact("MANIFEST.md", ArtifactDocument, content, "Artifact manifest", "system")
}
