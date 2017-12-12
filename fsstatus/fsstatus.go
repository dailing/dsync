/*This package records the status of files.*/

package fsstatus

import (
	"github.com/dailing/levlog"
	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3"
	"time"
)

const DB_NAME = "FsStatus.db"

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
	LocalRootDir string
	FilePath     string
	Dirty        bool
	FootPrint    string
	LastChange   time.Time
}

type FileStatusService struct {
	sqlEngine *xorm.Engine
}

func NewFileStatus() *FileStatusService {
	var err error
	fss := &FileStatusService{}
	fss.sqlEngine, err = xorm.NewEngine("sqlite3", DB_NAME)
	levlog.F(err)
	err = fss.sqlEngine.Charset("utf8").Sync2(
		new(FileStatus),
		new(RootDir),
	)
	levlog.F(err)
	return fss
}

func (fss *FileStatusService) createOrUpdate(filePath string) error {
	levlog.Trace("Create Or Update : ", filePath)
	// TODO first match root dir

	// TODO find record from Sqlite

	// TODO get footprint of the file, possibly compare with the original file

	// TODO generate patch file?

	// TODO write back records

	return nil
}
