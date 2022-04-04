// Copyright 2022 The Sqlite Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sqlite // import "modernc.org/sqlite"

import (
	"fmt"
	"unsafe"

	"modernc.org/libc"
	"modernc.org/libc/sys/types"
	sqlite3 "modernc.org/sqlite/lib"
)

// int sqlite3_open_v2(
//   const char *filename,   /* Database filename (UTF-8) */
//   sqlite3 **ppDb,         /* OUT: SQLite db handle */
//   int flags,              /* Flags */
//   const char *zVfs        /* Name of VFS module to use */
// );
func openV2(tls *libc.TLS, name string, flags int32) (db uintptr, err error) {
	var p, s uintptr

	defer func() {
		if p != 0 {
			libc.Xfree(tls, p)
		}
		if s != 0 {
			libc.Xfree(tls, s)
		}
	}()

	p = libc.Xmalloc(tls, types.Size_t(ptrSize))
	if p == 0 {
		return 0, fmt.Errorf("sqlite: cannot allocate %d bytes of memory", ptrSize)
	}

	if s, err = libc.CString(name); err != nil {
		return 0, err
	}

	if rc := sqlite3.Xsqlite3_open_v2(tls, s, p, flags, 0); rc != sqlite3.SQLITE_OK {
		return 0, liberrrc(tls, rc)
	}
	return *(*uintptr)(unsafe.Pointer(p)), nil
}
