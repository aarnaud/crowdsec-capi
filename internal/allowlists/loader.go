package allowlists

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/aarnaud/crowdsec-central-api/internal/db/queries"
	"github.com/aarnaud/crowdsec-central-api/internal/models"
)

// File represents the top-level structure of an allowlists YAML file.
type File struct {
	Allowlists []AllowlistDef `yaml:"allowlists"`
}

// AllowlistDef defines one allowlist and its entries.
type AllowlistDef struct {
	Name        string     `yaml:"name"`
	Label       string     `yaml:"label"`
	Description string     `yaml:"description"`
	Entries     []EntryDef `yaml:"entries"`
}

// EntryDef defines a single allowlist entry.
type EntryDef struct {
	Scope   string `yaml:"scope"`
	Value   string `yaml:"value"`
	Comment string `yaml:"comment"`
}

// LoadFile reads the YAML file at path and upserts all defined allowlists into
// the database, replacing their entries with the file contents.
// Allowlists defined in the file are marked as managed=true and cannot be
// deleted through the API or UI.
func LoadFile(ctx context.Context, db *pgxpool.Pool, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading allowlists file %q: %w", path, err)
	}

	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parsing allowlists file %q: %w", path, err)
	}

	for _, def := range f.Allowlists {
		if def.Name == "" {
			return fmt.Errorf("allowlist entry missing required field 'name'")
		}

		id, err := queries.UpsertManagedAllowlist(ctx, db, def.Name, def.Label, def.Description)
		if err != nil {
			return fmt.Errorf("upserting allowlist %q: %w", def.Name, err)
		}

		entries := make([]models.AllowlistEntry, 0, len(def.Entries))
		for _, e := range def.Entries {
			if e.Scope == "" || e.Value == "" {
				log.Warn().Str("allowlist", def.Name).Msg("skipping entry with empty scope or value")
				continue
			}
			comment := e.Comment
			entries = append(entries, models.AllowlistEntry{
				Scope:   e.Scope,
				Value:   e.Value,
				Comment: &comment,
			})
		}

		if err := queries.SyncManagedEntries(ctx, db, id, entries); err != nil {
			return fmt.Errorf("syncing entries for allowlist %q: %w", def.Name, err)
		}

		log.Info().
			Str("name", def.Name).
			Int("entries", len(entries)).
			Msg("allowlist synced from file")
	}

	return nil
}
