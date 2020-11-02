package sqlite

import (
	"modernc.org/sqlite/lib"
)

/*
Example working backup
func main() {
	src := "database.sqlite"
	dest := "database.backup"

	var sqlite3dstConn *sqlite3.SQLiteConn
	sql.Register("sqlite3_backup_dst",
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				sqlite3dstConn = conn
				return nil
			},
		})

	// Connect to the destination database.
	destDb, err := sql.Open("sqlite3_backup_dst", dest)
	if err != nil {
		log.Fatalf("Failed to open the destination database:", err)
	}
	defer destDb.Close()

	err = destDb.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to the destination database:", err)
	}

	var sqlite3srcConn *sqlite3.SQLiteConn
	sql.Register("sqlite3_backup_src",
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				sqlite3srcConn = conn
				return nil
			},
		})

	// Connect to the destination database.
	srcDB, err := sql.Open("sqlite3_backup_src", src)
	if err != nil {
		log.Fatalf("Failed to open the source database:", err)
	}
	defer srcDB.Close()

	// Important
	err = srcDB.Ping()
	if err != nil {
		log.Fatalf("Failed to connect to the source database:", err)
	}

	bk, err := sqlite3dstConn.Backup("main", sqlite3srcConn, "main")
	if err != nil {
		log.Fatalf("Failed to connect to the source database:", err)
	}

 	// Backup entire db
	//_, err = bk.Step(-1)
	//if err != nil {
	//	log.Fatalf("Step %s", err)
	//}

	// Step Progress
    isDone := false
	for !isDone {
		// Perform the backup step.
		isDone, err = bk.Step(1)
		if err != nil {
			log.Fatalf("Failed to perform a backup step:", err)
		}
	}


	err = bk.Finish()
	if err != nil {
		log.Fatalf("Finish %s", err)
	}
}
*/

// SQLiteBackup implement interface of Backup.
type SQLiteBackup struct {
	b sqlite3.Sqlite3_backup
}

// Backup make backup from src to dest.
func (destConn *conn) Backup(dest string, srcConn *conn, src string) (*SQLiteBackup, error) {
	return nil, nil
}

// Step to backs up for one step. Calls the underlying `sqlite3_backup_step`
// function.  This function returns a boolean indicating if the backup is done
// and an error signalling any other error. Done is returned if the underlying
// C function returns SQLITE_DONE (Code 101)
func (b *SQLiteBackup) Step(p int) (bool, error) {
	return false, nil
}

// Remaining return whether have the rest for backup.
func (b *SQLiteBackup) Remaining() int {
	return 0
}

// PageCount return count of pages.
func (b *SQLiteBackup) PageCount() int {
	return 0
}

// Finish close backup.
func (b *SQLiteBackup) Finish() error {
	return b.Close()
}

// Close close backup.
func (b *SQLiteBackup) Close() error {
	return nil
}
