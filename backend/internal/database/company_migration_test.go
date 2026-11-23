package database

import (
	"path/filepath"
	"testing"

	"yingce/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestCompaniesAndFixedChannelsMigrationPreservesLegacyChannel(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        filepath.Join(t.TempDir(), "schema.db"),
	}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	for _, sql := range []string{
		"CREATE TABLE model_channels (id text PRIMARY KEY, name text NOT NULL)",
		"INSERT INTO model_channels (id, name) VALUES ('legacy', 'Existing channel')",
		"CREATE TABLE route_attempts (id text PRIMARY KEY)",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := db.Transaction(migrateCompaniesAndFixedChannels); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []any{&model.Company{}, &model.CompanyMember{}, &model.UserChannelCredential{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table for %T", table)
		}
	}
	var row struct {
		Name             string
		CredentialSource string
	}
	if err := db.Table("model_channels").Where("id = ?", "legacy").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Name != "Existing channel" || row.CredentialSource != "platform" {
		t.Fatalf("legacy channel changed: %+v", row)
	}
	if !db.Migrator().HasColumn(&model.RouteAttempt{}, "SubmissionCredentialCipher") {
		t.Fatal("route attempts missing submission credential column")
	}
}

func TestCompanyStatusMigrationEnablesExistingCompanies(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "status.db")}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	for _, sql := range []string{
		"CREATE TABLE companies (id text PRIMARY KEY, name text NOT NULL)",
		"INSERT INTO companies (id, name) VALUES ('legacy', 'Existing company')",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, migration := range schemaMigrations {
		if migration.name == "company_status" {
			if err := db.Transaction(migration.apply); err != nil {
				t.Fatal(err)
			}
			var company model.Company
			if err := db.First(&company, "id = ?", "legacy").Error; err != nil {
				t.Fatal(err)
			}
			if company.Status != model.UserStatusActive {
				t.Fatalf("legacy company status = %q, want active", company.Status)
			}
			return
		}
	}
	t.Fatal("company status migration missing")
}
