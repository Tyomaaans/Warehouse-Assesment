package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"

	"gorm.io/gorm"
)

//go:embed *.sql
var files embed.FS

func Check(ctx context.Context, db *gorm.DB) error {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	for _, name := range names {
		content, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		var checksum string
		if err := db.WithContext(ctx).Raw("SELECT checksum FROM schema_migrations WHERE name=?", name).Scan(&checksum).Error; err != nil {
			return err
		}
		if checksum != fmt.Sprintf("%x", sha256.Sum256(content)) {
			return fmt.Errorf("migration %s is missing or modified", name)
		}
	}
	return nil
}

func Run(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(88620)").Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(
id uuid PRIMARY KEY DEFAULT gen_random_uuid() CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b')),
name text NOT NULL UNIQUE, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`).Error; err != nil {
			return err
		}
		names, err := fs.Glob(files, "*.sql")
		if err != nil {
			return err
		}
		for _, name := range names {
			content, err := files.ReadFile(name)
			if err != nil {
				return err
			}
			checksum := fmt.Sprintf("%x", sha256.Sum256(content))
			var existing string
			if err := tx.Raw("SELECT checksum FROM schema_migrations WHERE name=?", name).Scan(&existing).Error; err != nil {
				return err
			}
			if existing != "" {
				if existing != checksum {
					return fmt.Errorf("migration %s was modified", name)
				}
				continue
			}
			if err := tx.Exec(string(content)).Error; err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
			if err := tx.Exec("INSERT INTO schema_migrations(name,checksum) VALUES (?,?)", name, checksum).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
