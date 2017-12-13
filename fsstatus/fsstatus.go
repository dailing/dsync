/*This package records the status of files.*/

package fsstatus

import (
	"errors"
	"fmt"
	"github.com/dailing/levlog"
	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3"
	"path/filepath"
	"time"
)

type RootDir struct {
	Id   int64
	Path string `xorm:"Unique"`
}

type RootDirFilters struct {
	Id           int64
	LocalRootDir int64
	RegExp       string
}

type FileStatus struct {
	Id           int64
	LocalRootDir int64
	FilePath     string
	Dirty        bool
	FootPrint    string
	LastChange   time.Time
}

type FileStatusService struct {
	sqlEngine *xorm.Engine
}

func NewFileStatus(dbname string) *FileStatusService {
	var err error
	fss := &FileStatusService{}
	fss.sqlEngine, err = xorm.NewEngine("sqlite3", dbname)
	levlog.F(err)
	err = fss.sqlEngine.Charset("utf8").Sync2(
		new(FileStatus),
		new(RootDir),
	)
	levlog.F(err)
	return fss
}

func (fss *FileStatusService) CreateRootEntry(filePath string) error {
	if !filepath.IsAbs(filePath) {
		return errors.New("path " + filePath + " is not a abs path")
	}
	_, err := fss.sqlEngine.Insert(RootDir{
		Path: filePath,
	})
	levlog.E(err)
	return err
}

func (fss *FileStatusService) CreateOrUpdate(filePath string) error {
	levlog.Trace("Create Or Update : ", filePath)
	// Note that the filePath has to be absolute path. Otherwise it cannot match the records in db.
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		return errors.New("not absolute path")
	}
	// search for the root dir entry in db
	// default match the longest one.
	rootDir := RootDir{}
	sqlQuery := fmt.Sprintf(
		`SELECT *, instr('%s',root_dir.path) as startNum FROM root_dir WHERE startNum > 0 ORDER BY length(path)`,
		filePath,
	)
	b, err := fss.sqlEngine.SQL(sqlQuery, filePath).Get(&rootDir)
	levlog.Trace(sqlQuery)
	levlog.Trace("find root dir:", rootDir.Path)
	levlog.E(err)
	if !b {
		return errors.New("cannot find root directory entry")
	}

	// get the relative path of the file
	relPath := filePath[len(rootDir.Path)+1:]
	levlog.Info(relPath)

	// TODO find record from Sqlite, check this
	fileStatus := FileStatus{
		LocalRootDir: rootDir.Id,
		FilePath:     relPath,
		LastChange:   time.Now().Add(-time.Hour),
	}
	fileRecExist, err := fss.sqlEngine.
		SQL("Select * from file_status where file_path=? and local_root_dir=?",
		relPath, rootDir.Id).
		Get(&fileStatus)

	levlog.E(err)
	levlog.Trace("File Exist:", fileRecExist)
	levlog.Trace(rootDir.Id)
	levlog.Trace(relPath)
	levlog.Trace(fileStatus)

	// TODO get footprint of the file, possibly compare with the original file
	fileStatus.LastChange = time.Now()
	if fileRecExist {
		levlog.Trace("Updating records")
		_, err := fss.sqlEngine.Id(fileStatus.Id).Update(&fileStatus)
		levlog.E(err)
	} else {
		_, err := fss.sqlEngine.Insert(&fileStatus)
		levlog.E(err)
	}

	// TODO generate patch file?

	// TODO write back records

	return nil
}
