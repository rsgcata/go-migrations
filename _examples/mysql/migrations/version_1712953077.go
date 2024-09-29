package migrations

import (
	"database/sql"
)

type Migration1712953077 struct {
	Db *sql.DB
}

func (migration *Migration1712953077) Version() uint64 {
	return 1712953077
}

func (migration *Migration1712953077) Up() error {
	_, err := migration.Db.Exec("create table if not exists `users` (`id` integer unsigned auto_increment not null, `name` varchar(128) not null, `phone` varchar(32) not null, primary key (`id`))")
	return err
}

func (migration *Migration1712953077) Down() error {
	_, err := migration.Db.Exec("drop table if exists `users`")
	return err
}
