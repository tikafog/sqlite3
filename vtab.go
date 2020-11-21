// Copyright 2020 The Sqlite Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sqlite // import "modernc.org/sqlite"

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"modernc.org/libc"
	"modernc.org/libc/sys/types"
	"modernc.org/sqlite/lib"
)

var (
	_ VTaber = (*conn)(nil)
)

type registeredModule struct {
	m Module
	p uintptr // *sqlite3.Sqlite3_module
}

func origin(skip int) string {
	pc, fn, fl, _ := runtime.Caller(skip)
	f := runtime.FuncForPC(pc)
	var fns string
	if f != nil {
		fns = f.Name()
		if x := strings.LastIndex(fns, "."); x > 0 {
			fns = fns[x+1:]
		}
	}
	return fmt.Sprintf("%s:%d:%s", fn, fl, fns)
}

func todo(s string, args ...interface{}) string {
	switch {
	case s == "":
		s = fmt.Sprintf(strings.Repeat("%v ", len(args)), args...)
	default:
		s = fmt.Sprintf(s, args...)
	}
	return fmt.Sprintf("%s: TODO %s", origin(2), s) //TODOOK
}

func trc(s string, args ...interface{}) string {
	switch {
	case s == "":
		s = fmt.Sprintf(strings.Repeat("%v ", len(args)), args...)
	default:
		s = fmt.Sprintf(s, args...)
	}
	r := fmt.Sprintf("%s: TRC %s", origin(2), s)
	fmt.Fprintf(os.Stdout, "%s\n", r)
	os.Stdout.Sync()
	return r
}

// These routines are used to register a new virtual table module name. Module
// names must be registered before creating a new virtual table using the
// module and before using a preexisting virtual table for the module.
//
// The module name is registered on the database connection specified by the
// first parameter. The name of the module is given by the second parameter.
// The third parameter is a pointer to the implementation of the virtual table
// module. The fourth parameter is an arbitrary client data pointer that is
// passed through into the xCreate and xConnect methods of the virtual table
// module when a new virtual table is be being created or reinitialized.
//
// If the third parameter (the pointer to the sqlite3_module object) is NULL
// then no new module is create and any existing modules with the same name are
// dropped.
//
//	int sqlite3_create_module(
//	  sqlite3 *db,               /* SQLite connection to register module with */
//	  const char *zName,         /* Name of the module */
//	  const sqlite3_module *p,   /* Methods for the module */
//	  void *pClientData          /* Client data for xCreate/xConnect */
//	);
func create_module(tls *libc.TLS, db uintptr, zName string, p, pClientData uintptr) error {
	czName, err := libc.CString(zName)
	if err != nil {
		return err
	}

	defer libc.Xfree(tls, czName)

	if rc := sqlite3.Xsqlite3_create_module(tls, db, czName, p, pClientData); rc != sqlite3.SQLITE_OK {
		return errstr(tls, db, rc)
	}

	return nil
}

// const char *sqlite3_errstr(int);
func errstr(tls *libc.TLS, db uintptr, rc int32) error {
	p := sqlite3.Xsqlite3_errstr(tls, rc)
	str := libc.GoString(p)
	p = sqlite3.Xsqlite3_errmsg(tls, db)
	switch msg := libc.GoString(p); {
	case msg == str:
		return &Error{msg: fmt.Sprintf("%s (%v)", str, rc), code: int(rc)}
	default:
		return &Error{msg: fmt.Sprintf("%s: %s (%v)", str, msg, rc), code: int(rc)}
	}
}

func createModule(tls *libc.TLS, db uintptr, zName string, m uintptr) error {
	return create_module(tls, db, zName, m, 0)
}

// VTaber is an optional interface of database/sql/driver.Driver
type VTaber interface {
	CreateModule(name string, m Module) error
}

// CreateModule registers a new virtual table module name. Module names must be
// registered before creating a new virtual table using the module and before
// using a preexisting virtual table for the module.  If m is nil then no new
// module is create and any existing modules with the same name are dropped.
func (c *conn) CreateModule(name string, m Module) error {
	p, err := newModule(c.tls, m)
	if err != nil {
		return err
	}

	if err := createModule(c.tls, c.db, name, p); err != nil {
		return err
	}

	switch {
	case m == nil:
		libc.Xfree(c.tls, c.modules[name].p)
		delete(c.modules, name)
	default:
		c.modules[name] = registeredModule{m, p}
	}
	return nil
}

type Module interface {
	// Version reports the particular edition of the module.
	Version() int
}

func newModule(tls *libc.TLS, m Module) (p uintptr, err error) {
	if m == nil {
		return 0, nil
	}

	p = libc.Xcalloc(tls, 1, types.Size_t(unsafe.Sizeof(sqlite3.Sqlite3_module{})))
	*(*sqlite3.Sqlite3_module)(unsafe.Pointer(p)) = sqlite3.Sqlite3_module{
		//	  int iVersion;
		FiVersion: int32(m.Version()),
		//	  int (*xCreate)(sqlite3*, void *pAux,
		//	               int argc, char *const*argv,
		//	               sqlite3_vtab **ppVTab,
		//	               char **pzErr);
		FxCreate: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, uintptr, int32, uintptr, uintptr, uintptr) int32
		}{xCreate})),
		//	  int (*xConnect)(sqlite3*, void *pAux,
		//	               int argc, char *const*argv,
		//	               sqlite3_vtab **ppVTab,
		//	               char **pzErr);
		FxConnect: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, uintptr, int32, uintptr, uintptr, uintptr) int32
		}{xConnect})),
		//	  int (*xBestIndex)(sqlite3_vtab *pVTab, sqlite3_index_info*);
		FxBestIndex: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, uintptr) int32
		}{xBestIndex})),
		//	  int (*xDisconnect)(sqlite3_vtab *pVTab);
		FxDisconnect: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xDisconnect})),
		//	  int (*xDestroy)(sqlite3_vtab *pVTab);
		FxDestroy: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xDestroy})),
		//	  int (*xOpen)(sqlite3_vtab *pVTab, sqlite3_vtab_cursor **ppCursor);
		FxOpen: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, uintptr) int32
		}{xOpen})),
		//	  int (*xClose)(sqlite3_vtab_cursor*);
		FxClose: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xClose})),
		//	  int (*xFilter)(sqlite3_vtab_cursor*, int idxNum, const char *idxStr,
		//	                int argc, sqlite3_value **argv);
		FxFilter: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, int32, uintptr, int32, uintptr) int32
		}{xFilter})),
		//	  int (*xNext)(sqlite3_vtab_cursor*);
		FxNext: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xNext})),
		//	  int (*xEof)(sqlite3_vtab_cursor*);
		FxEof: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xEof})),
		//	  int (*xColumn)(sqlite3_vtab_cursor*, sqlite3_context*, int);
		FxColumn: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, uintptr, int32) int32
		}{xColumn})),
		//	  int (*xRowid)(sqlite3_vtab_cursor*, sqlite_int64 *pRowid);
		FxRowid: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, uintptr) int32
		}{xRowid})),
		//	  int (*xUpdate)(sqlite3_vtab *, int, sqlite3_value **, sqlite_int64 *);
		FxUpdate: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, int32, uintptr, uintptr) int32
		}{xUpdate})),
		//	  int (*xBegin)(sqlite3_vtab *pVTab);
		FxBegin: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xBegin})),
		//	  int (*xSync)(sqlite3_vtab *pVTab);
		FxSync: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xSync})),
		//	  int (*xCommit)(sqlite3_vtab *pVTab);
		FxCommit: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xCommit})),
		//	  int (*xRollback)(sqlite3_vtab *pVTab);
		FxRollback: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xRollback})),
		//	  int (*xFindFunction)(sqlite3_vtab *pVtab, int nArg, const char *zName,
		//	                     void (**pxFunc)(sqlite3_context*,int,sqlite3_value**),
		//	                     void **ppArg);
		FxFindFunction: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, int32, uintptr, uintptr, uintptr) int32
		}{xFindFunction})),
		//	  int (*Rename)(sqlite3_vtab *pVtab, const char *zNew);
		FxRename: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, uintptr) int32
		}{xRename})),
		//	  /* The methods above are in version 1 of the sqlite_module object. Those
		//	  ** below are for version 2 and greater. */
		//	  int (*xSavepoint)(sqlite3_vtab *pVTab, int);
		FxSavepoint: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, int32) int32
		}{xSavepoint})),
		//	  int (*xRelease)(sqlite3_vtab *pVTab, int);
		FxRelease: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, int32) int32
		}{xRelease})),
		//	  int (*xRollbackTo)(sqlite3_vtab *pVTab, int);
		FxRollbackTo: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr, int32) int32
		}{xRollbackTo})),
		//	  /* The methods above are in versions 1 and 2 of the sqlite_module object.
		//	  ** Those below are for version 3 and greater. */
		//	  int (*xShadowName)(const char*);
		FxShadowName: *(*uintptr)(unsafe.Pointer(&struct {
			f func(*libc.TLS, uintptr) int32
		}{xShadowName})),
	}
	return p, nil
}

// Create a virtual table.
//
//	  int (*xCreate)(sqlite3*, void *pAux,
//	               int argc, char *const*argv,
//	               sqlite3_vtab **ppVTab,
//	               char **pzErr);
func xCreate(tls *libc.TLS, db uintptr, pAux uintptr, argc int32, argv uintptr, ppVtab uintptr, pzErr uintptr) int32 {
	panic(todo(""))
}

// Connect to a virtual table.
//
//	  int (*xConnect)(sqlite3*, void *pAux,
//	               int argc, char *const*argv,
//	               sqlite3_vtab **ppVTab,
//	               char **pzErr);
func xConnect(tls *libc.TLS, db uintptr, pAux uintptr, argc int32, argv uintptr, ppVtab uintptr, pzErr uintptr) int32 {
	panic(todo(""))
}

//	  int (*xBestIndex)(sqlite3_vtab *pVTab, sqlite3_index_info*);
func xBestIndex(tls *libc.TLS, tab uintptr, pIdxInfo uintptr) int32 {
	panic(todo(""))
}

// Disconnect from a virtual table.
//
//	  int (*xDisconnect)(sqlite3_vtab *pVTab);
func xDisconnect(tls *libc.TLS, pVtab uintptr) int32 {
	panic(todo(""))
}

// Destroy a virtual table.
//
//	  int (*xDestroy)(sqlite3_vtab *pVTab);
func xDestroy(tls *libc.TLS, pVtab uintptr) int32 {
	panic(todo(""))
}

// Open a new vtab cursor.
//
//	  int (*xOpen)(sqlite3_vtab *pVTab, sqlite3_vtab_cursor **ppCursor);
func xOpen(tls *libc.TLS, pVTab uintptr, ppCursor uintptr) int32 {
	panic(todo(""))
}

// Close a vtab cursor.
//
//	  int (*xClose)(sqlite3_vtab_cursor*);
func xClose(tls *libc.TLS, pCursor uintptr) int32 {
	panic(todo(""))
}

//	  int (*xFilter)(sqlite3_vtab_cursor*, int idxNum, const char *idxStr,
//	                int argc, sqlite3_value **argv);
func xFilter(tls *libc.TLS, pCursor uintptr, idxNum int32, idxStr uintptr, argc int32, argv uintptr) int32 {
	panic(todo(""))
}

// Move a vtab cursor to the next entry.
//
//	  int (*xNext)(sqlite3_vtab_cursor*);
func xNext(tls *libc.TLS, pCursor uintptr) int32 {
	panic(todo(""))
}

//	  int (*xEof)(sqlite3_vtab_cursor*);
func xEof(tls *libc.TLS, pCursor uintptr) int32 {
	panic(todo(""))
}

//	  int (*xColumn)(sqlite3_vtab_cursor*, sqlite3_context*, int);
func xColumn(tls *libc.TLS, pCursor uintptr, ctx uintptr, i int32) int32 {
	panic(todo(""))
}

//	  int (*xRowid)(sqlite3_vtab_cursor*, sqlite_int64 *pRowid);
func xRowid(tls *libc.TLS, pCursor uintptr, pRowid uintptr) int32 {
	panic(todo(""))
}

//	  int (*xUpdate)(sqlite3_vtab *, int, sqlite3_value **, sqlite_int64 *);
func xUpdate(tls *libc.TLS, pVtab uintptr, argc int32, argv uintptr, pRowid uintptr) int32 {
	panic(todo(""))
}

//	  int (*xBegin)(sqlite3_vtab *pVTab);
func xBegin(tls *libc.TLS, pVtab uintptr) int32 {
	panic(todo(""))
}

//	  int (*xSync)(sqlite3_vtab *pVTab);
func xSync(tls *libc.TLS, pVtab uintptr) int32 {
	panic(todo(""))
}

//	  int (*xCommit)(sqlite3_vtab *pVTab);
func xCommit(tls *libc.TLS, pVtab uintptr) int32 {
	panic(todo(""))
}

//	  int (*xRollback)(sqlite3_vtab *pVTab);
func xRollback(tls *libc.TLS, pVtab uintptr) int32 {
	panic(todo(""))
}

//	  int (*xFindFunction)(sqlite3_vtab *pVtab, int nArg, const char *zName,
//	                     void (**pxFunc)(sqlite3_context*,int,sqlite3_value**),
//	                     void **ppArg);
func xFindFunction(tls *libc.TLS, pVtab uintptr, nUnused int32, zName uintptr, pxFunc uintptr, ppArg uintptr) int32 {
	panic(todo(""))
}

//	  int (*Rename)(sqlite3_vtab *pVtab, const char *zNew);
func xRename(tls *libc.TLS, pVtab uintptr, zName uintptr) int32 {
	panic(todo(""))
}

//	  int (*xSavepoint)(sqlite3_vtab *pVTab, int);
func xSavepoint(tls *libc.TLS, pVtab uintptr, iSavepoint int32) int32 {
	panic(todo(""))
}

//	  int (*xRelease)(sqlite3_vtab *pVTab, int);
func xRelease(tls *libc.TLS, pVtab uintptr, iSavepoint int32) int32 {
	panic(todo(""))
}

//	  int (*xRollbackTo)(sqlite3_vtab *pVTab, int);
func xRollbackTo(tls *libc.TLS, pVtab uintptr, iSavepoint int32) int32 {
	panic(todo(""))
}

//	  int (*xShadowName)(const char*);
func xShadowName(tls *libc.TLS, zName uintptr) int32 {
	panic(todo(""))
}
