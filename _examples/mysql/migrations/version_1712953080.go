package migrations

import (
	"database/sql"
)

type Migration1712953080 struct {
	Db *sql.DB
}

func (migration *Migration1712953080) Version() uint64 {
	return 1712953080
}

func (migration *Migration1712953080) Up() error {
	_, err := migration.Db.Exec("alter table `users` change `phone` `phone_num` varchar(64) not null")
	return err
}

func (migration *Migration1712953080) Down() error {
	_, err := migration.Db.Exec("alter table `users` change `phone_num` `phone` varchar(32) not null")
	return err
}
