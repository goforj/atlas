package diagnostics

import "testing"

func TestValidateReadOnlySQLAllowsReads(t *testing.T) {
	for _, query := range []string{
		"select * from users",
		"WITH active AS (SELECT * FROM users) SELECT * FROM active",
		"show tables",
		"describe users",
		"explain select * from users",
	} {
		if err := ValidateReadOnlySQL(query); err != nil {
			t.Fatalf("expected read query to pass %q: %v", query, err)
		}
	}
}

func TestValidateReadOnlySQLRejectsMutations(t *testing.T) {
	for _, query := range []string{
		"insert into users values (1)",
		"update users set name = 'x'",
		"delete from users",
		"drop table users",
		"with deleted as (delete from users returning *) select * from deleted",
	} {
		if err := ValidateReadOnlySQL(query); err == nil {
			t.Fatalf("expected mutation query to fail %q", query)
		}
	}
}
