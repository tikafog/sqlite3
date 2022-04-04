// Copyright 2022 The Sqlite Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sqlite // modernc.org/sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"modernc.org/libc"
)

func TestBackup(t *testing.T) {
	ctx := context.Background()

	srcDsn := filepath.Join(t.TempDir(), "src.db")
	destDsn := filepath.Join(t.TempDir(), "dst.db")

	srcDb, err := sql.Open(driverName, srcDsn)
	if err != nil {
		t.Fatal(err)
	}
	defer srcDb.Close()

	destDb, err := sql.Open(driverName, destDsn)
	if err != nil {
		t.Fatal(err)
	}
	defer destDb.Close()

	if _, err := srcDb.Exec("create table t(b int)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := srcDb.Exec("insert into t values (?)", i); err != nil {
			t.Fatal(err)
		}
	}

	libc.MemAuditStart()
	err = Backup(ctx, destDsn, srcDsn)
	if merr := libc.MemAuditReport(); merr != nil {
		t.Error(merr)
	}
	if err != nil {
		t.Fatal(err)
	}

	rows, err := destDb.Query("select b from t")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for i := 0; i < 1000; i++ {
		if !rows.Next() {
			t.Fatal("expected at least one result row")
		}

		var a int
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		if a != i {
			t.Fatal("expected to read back the expected content after backup")
		}
	}
	if rows.Next() {
		t.Fatal("expected no more result rows")
	}
}
