package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

type Migration1712953083 struct {
	Db  *sql.DB
	Ctx context.Context
}

func (migration *Migration1712953083) Version() uint64 {
	return 1712953083
}

func (migration *Migration1712953083) Up() error {
	tx, err := migration.Db.BeginTx(migration.Ctx, nil)

	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		migration.Ctx,
		"insert into `users` (`name`, `phone_num`) values ('Alex', '1234'), ('Jada', '4567'), ('Tia', '7890')",
	)

	if err != nil {
		errRollback := tx.Rollback()

		if errRollback != nil {
			return fmt.Errorf("%w, with rollback error: %w", err, errRollback)
		}
		return err
	}

	return tx.Commit()
}

func (migration *Migration1712953083) Down() error {
	tx, err := migration.Db.BeginTx(migration.Ctx, nil)

	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		migration.Ctx,
		"delete from `users` where `name` in ('Alex', 'Jada', 'Tia')",
	)

	if err != nil {
		errRollback := tx.Rollback()

		if errRollback != nil {
			return fmt.Errorf("%w, with rollback error: %w", err, errRollback)
		}
		return err
	}

	return tx.Commit()
}
