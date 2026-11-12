package user

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// GetBalance возвращает начисления по PROCESSED минус списания и общую сумму списаний.
// При отсутствии записей суммы равны нулю.
func (userRepository Repository) GetBalance(ctx context.Context, userID int64) (float64, float64, error) {
	var current, withdrawn float64

	err := userRepository.pool.QueryRow(
		ctx,
		`SELECT accrued - withdrawn AS current, withdrawn
		 FROM (
		     SELECT
		         COALESCE(
		             (SELECT SUM(bonusesCredited) FROM orders
		              WHERE userId = $1 AND status = 'PROCESSED'), 0
		         ) AS accrued,
		         COALESCE(
		             (SELECT SUM(sum) FROM withdrawals WHERE userId = $1), 0
		         ) AS withdrawn
		 ) totals`,
		userID,
	).Scan(&current, &withdrawn)

	return current, withdrawn, err
}

// GetWithdrawals возвращает списания userID от новых к старым по времени обработки.
func (userRepository Repository) GetWithdrawals(ctx context.Context, userID int64) ([]Withdrawal, error) {
	rows, err := userRepository.pool.Query(
		ctx,
		`SELECT id, userId, orderId, sum, processedAt
		 FROM withdrawals WHERE userId = $1 ORDER BY processedAt DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var withdrawals []Withdrawal
	for rows.Next() {
		var withdrawal Withdrawal
		if err := rows.Scan(&withdrawal.id, &withdrawal.userID, &withdrawal.orderID, &withdrawal.sum, &withdrawal.processedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	return withdrawals, rows.Err()
}

// Withdraw сохраняет списание или возвращает ErrInsufficientFunds при нехватке баллов.
// Блокировка пользователя исключает параллельный перерасход: остаток проверяется
// отдельным запросом после блокировки в Read Committed, суммы сравниваются как NUMERIC.
func (userRepository Repository) Withdraw(ctx context.Context, withdrawal Withdrawal) error {
	tx, err := userRepository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID int64
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, withdrawal.userID).Scan(&userID)
	if err != nil {
		return err
	}

	result, err := tx.Exec(
		ctx,
		`INSERT INTO withdrawals (userId, orderId, sum)
		 SELECT $1, $2, $3::numeric
		 WHERE $3::numeric <=
		     COALESCE(
		         (SELECT SUM(bonusesCredited) FROM orders
		          WHERE userId = $1 AND status = 'PROCESSED'), 0
		     ) - COALESCE(
		         (SELECT SUM(sum) FROM withdrawals WHERE userId = $1), 0
		     )`,
		withdrawal.userID, withdrawal.orderID, withdrawal.sum,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrInsufficientFunds
	}

	return tx.Commit(ctx)
}
