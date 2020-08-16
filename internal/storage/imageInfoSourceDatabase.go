// +build gorm

package photofield

import (
	. "photofield/internal"
	"time"

	"github.com/jinzhu/gorm"
)

type ImageInfoSourceDatabase struct {
	db             *gorm.DB
	dbPendingInfos chan *ImageInfoDb
}

type ImageInfoDb struct {
	Path      string `gorm:"type:varchar(4096);primary_key;unique_index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	ImageInfo
}

func NewImageInfoSourceDatabase() *ImageInfoSourceDatabase {
	var err error
	source := ImageInfoSourceDatabase{}

	db, err := gorm.Open("sqlite3", "data/photofield.cache.db")
	if err != nil {
		panic("failed to connect database")
	}
	source.db = db

	// Migrate the schema
	db.AutoMigrate(&ImageInfoDb{})

	source.dbPendingInfos = make(chan *ImageInfoDb, 100)
	go writePendingInfos(source.dbPendingInfos, db)

	return &source
}

func writePendingInfos(pendingInfos chan *ImageInfoDb, db *gorm.DB) {
	for imageInfo := range pendingInfos {
		db.Where("path = ?", imageInfo.Path).
			Assign(imageInfo).
			Assign(imageInfo.ImageInfo).
			FirstOrCreate(imageInfo)
	}
}

func (source *ImageInfoSourceDatabase) Get(path string) (*ImageInfo, error) {

	var imageInfoDb ImageInfoDb
	source.db.First(&imageInfoDb, "path = ?", path)

	valid := true
	if imageInfoDb.Path == "" {
		valid = false
	}

	// if strings.Contains(imageInfoDb.Path, "USA 2018") {
	// if strings.Contains(imageInfoDb.Path, "20180825_180816") {
	// 	valid = false
	// }

	// if imageInfoDb.ImageInfo.Width == 0 || imageInfoDb.ImageInfo.Height == 0 {
	// 	valid = false
	// }
	// if imageInfoDb.ImageInfo.DateTime.IsZero() {
	// 	valid = false
	// }
	// if imageInfoDb.ImageInfo.Color == 0 {
	// 	valid = false
	// }

	if valid {
		return &imageInfoDb.ImageInfo, nil
	}
	return nil, nil
}

func (source *ImageInfoSourceDatabase) Set(path string, info *ImageInfo) error {
	source.dbPendingInfos <- &ImageInfoDb{
		Path:      path,
		ImageInfo: *info,
	}
	return nil
}
