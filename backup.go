// Copyright 2022 The Sqlite Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sqlite // import "modernc.org/sqlite"

import (
	"context"
	"fmt"
	"time"

	"modernc.org/libc"
	sqlite3 "modernc.org/sqlite/lib"
)

// Backup uses the sqlite online [backup api](https://www.sqlite.org/c3ref/backup_finish.html)
// to copy the contents of srcDsn to destDsn.
//
// Per default 50 pages are copied every step with a backoff of 250ms from the 'main' database
// of the source to the 'main' database of destination. Every step a 'ProgressFunc' callback
// is invoked to track progress.
func Backup(ctx context.Context, destDsn, srcDsn string, opts ...BackupOption) (err error) {
	cfg := defaultBackupConfig()
	for i := range opts {
		opts[i](&cfg)
	}

	backup, err := newBackup(destDsn, cfg.destDb, srcDsn, cfg.srcDb)
	if err != nil {
		return fmt.Errorf("unable to initiate backup: %v", err)
	}
	defer func() {
		if cerr := backup.close(); err == nil {
			err = cerr
		}
	}()

L:
	for ; ; time.Sleep(cfg.sleepPerStep) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := backup.step(cfg.pagesPerStep); err != nil {
			liberr, ok := err.(*Error)
			if !ok {
				return err
			}
			switch liberr.Code() {
			case sqlite3.SQLITE_DONE:
				cfg.progressFunc(backup.remaining(), backup.pagecount())
				break L
			case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
				cfg.progressFunc(backup.remaining(), backup.pagecount())
				continue
			default:
				return liberr
			}
		}
		cfg.progressFunc(backup.remaining(), backup.pagecount())
	}
	return nil
}

// ProgressFunc is a callback that is evoked at every step of the backup.
type ProgressFunc func(remaining int32, pagecount int32)

type backupConfig struct {
	destDb string
	srcDb  string

	sleepPerStep time.Duration
	pagesPerStep int32
	progressFunc ProgressFunc
}

func defaultBackupConfig() backupConfig {
	return backupConfig{
		sleepPerStep: 250 * time.Millisecond,
		pagesPerStep: 50,
		progressFunc: func(remaining, pagecount int32) {},
		destDb:       "main",
		srcDb:        "main",
	}
}

// BackupOption is a way to configure the backup
type BackupOption func(*backupConfig)

// WithSleepPerStep causes the backup to sleep for the configured amount of time
// between steps. If not provided with this option, the default is 250ms.
func WithSleepPerStep(d time.Duration) BackupOption {
	return func(bc *backupConfig) {
		bc.sleepPerStep = d
	}
}

// WithSleepPerStep causes the backup to copy the configured amount of pages
// every step. If not provided with this option, the default is 50 pages.
func WithPagesPerStep(p int32) BackupOption {
	return func(bc *backupConfig) {
		bc.pagesPerStep = p
	}
}

// WithSleepPerStep configures the callback to invoke after every step to
// track progress of the backup. If not provided with this option, the
// default is a noop function.
func WithProgressFunc(f ProgressFunc) BackupOption {
	return func(bc *backupConfig) {
		bc.progressFunc = f
	}
}

// WithDestinationDatabase configures the database that is targeted with the backup.
// If not provided with this option, the default is 'main'.
func WithDestinationDatabase(db string) BackupOption {
	return func(bc *backupConfig) {
		bc.destDb = db
	}
}

// WithSourceDatabase configures the database that is to be backed up. If not
// provided with this option, the default is 'main'.
func WithSourceDatabase(db string) BackupOption {
	return func(bc *backupConfig) {
		bc.srcDb = db
	}
}

type backup struct {
	tls *libc.TLS

	backupHandle uintptr

	srcHandle  uintptr
	srcName    uintptr
	destHandle uintptr
	destName   uintptr
}

func newBackup(destDsn, destDb, srcDsn, srcDb string) (*backup, error) {
	const (
		destFlags = sqlite3.SQLITE_OPEN_READWRITE | sqlite3.SQLITE_OPEN_CREATE | sqlite3.SQLITE_OPEN_URI
		srcFlags  = sqlite3.SQLITE_OPEN_READONLY | sqlite3.SQLITE_OPEN_URI
	)
	var err error

	tls := libc.NewTLS()

	freeOnError := func(p uintptr) {
		if err != nil {
			libc.Xfree(tls, p)
		}
	}

	destHandle, err := openV2(tls, destDsn, destFlags)
	if err != nil {
		return nil, fmt.Errorf("unable to open dest db: %s", err)
	}
	defer freeOnError(destHandle)

	destName, err := libc.CString(destDb)
	if err != nil {
		return nil, fmt.Errorf("unable to convert dest db name to c string: %s", err)
	}
	defer freeOnError(destName)

	srcHandle, err := openV2(tls, srcDsn, srcFlags)
	if err != nil {
		return nil, fmt.Errorf("unable to open src db: %s", err)
	}
	defer freeOnError(srcHandle)

	srcName, err := libc.CString(srcDb)
	if err != nil {
		return nil, fmt.Errorf("unable to convert src db name to c string: %s", err)
	}
	defer freeOnError(srcName)

	backupHandle := sqlite3.Xsqlite3_backup_init(tls, destHandle, destName, srcHandle, srcName)
	if backupHandle == 0 {
		return nil, liberrdbrc(tls, destHandle, errcode(tls, destHandle))
	}

	return &backup{
		tls:          tls,
		backupHandle: backupHandle,
		srcHandle:    srcHandle,
		srcName:      srcName,
		destHandle:   destHandle,
		destName:     destName,
	}, nil
}

func (b *backup) close() (err error) {
	defer b.tls.Close()
	defer libc.Xfree(b.tls, b.destName)
	defer libc.Xfree(b.tls, b.srcName)

	if rc := sqlite3.Xsqlite3_backup_finish(b.tls, b.backupHandle); rc != sqlite3.SQLITE_OK {
		err = liberrdbrc(b.tls, b.destHandle, rc)
	}
	if rc := sqlite3.Xsqlite3_close_v2(b.tls, b.srcHandle); rc != sqlite3.SQLITE_OK && err == nil {
		err = liberrdbrc(b.tls, b.srcHandle, rc)
	}
	if rc := sqlite3.Xsqlite3_close_v2(b.tls, b.destHandle); rc != sqlite3.SQLITE_OK && err == nil {
		err = liberrdbrc(b.tls, b.destHandle, rc)
	}
	return err
}

func (b *backup) step(nPage int32) error {
	if rc := sqlite3.Xsqlite3_backup_step(b.tls, b.backupHandle, nPage); rc != sqlite3.SQLITE_OK {
		return liberrdbrc(b.tls, b.destHandle, rc)
	}
	return nil
}

func (b *backup) remaining() int32 {
	return sqlite3.Xsqlite3_backup_remaining(b.tls, b.backupHandle)
}

func (b *backup) pagecount() int32 {
	return sqlite3.Xsqlite3_backup_pagecount(b.tls, b.backupHandle)
}
