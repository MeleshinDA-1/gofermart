package order

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository читает и сохраняет заказы в PostgreSQL.
type Repository struct {
	pool ordersDB
}

type ordersDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
}

// GetOrders возвращает только заказы userID, начиная с самых поздних загрузок.
func (o *Repository) GetOrders(ctx context.Context, userID int64) ([]Order, error) {
	rows, err := o.pool.Query(ctx,
		`SELECT orderId, userId, status, COALESCE(bonusesCredited, 0), uploadDate
		 FROM orders WHERE userId = $1 ORDER BY uploadDate DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		var status string
		if err := rows.Scan(&order.orderID, &order.userID, &status, &order.bonusesCredited, &order.uploadDate); err != nil {
			return nil, err
		}
		order.status, err = parseStatus(status)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// SaveOrder вставляет заказ, не изменяя запись с уже занятым номером.
// При конфликте возвращает ErrRepeatingOrder для своего заказа или ErrForeignOrder для чужого.
func (o *Repository) SaveOrder(ctx context.Context, order Order) error {
	result, err := o.pool.Exec(
		ctx,
		`INSERT INTO orders (orderId, userId, status, bonusesCredited, uploadDate)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (orderHash) DO NOTHING`,
		order.orderID, order.userID, order.status.String(), order.bonusesCredited, order.uploadDate,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 1 {
		return nil
	}

	var ownerID int64
	err = o.pool.QueryRow(ctx,
		`SELECT userId FROM orders WHERE orderHash = sha256($1::text::bytea) AND orderId = $1`,
		order.orderID,
	).Scan(&ownerID)
	if err != nil {
		return err
	}
	if ownerID == order.userID {
		return ErrRepeatingOrder
	}
	return ErrForeignOrder
}

// GetNotFinished возвращает номера заказов со статусами NEW и PROCESSING.
func (o *Repository) GetNotFinished(ctx context.Context) ([]string, error) {
	rows, err := o.pool.Query(ctx,
		`SELECT orderId FROM orders WHERE status IN ('NEW', 'PROCESSING') ORDER BY uploadDate`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orderIDs []string
	for rows.Next() {
		var orderID string
		if err := rows.Scan(&orderID); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, orderID)
	}
	return orderIDs, rows.Err()
}

// BulkUpdate записывает статусы и начисления одним пакетом запросов.
// Заказы с окончательными статусами не изменяются, суммы заменяются без сложения.
func (o *Repository) BulkUpdate(ctx context.Context, orders []Order) error {
	if len(orders) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, order := range orders {
		batch.Queue(
			`UPDATE orders SET status = $2, bonusesCredited = $3
			 WHERE orderHash = sha256($1::text::bytea) AND orderId = $1
			 AND status IN ('NEW', 'PROCESSING')`,
			order.orderID, order.status.String(), order.bonusesCredited,
		)
	}
	return o.pool.SendBatch(ctx, batch).Close()
}

// NewRepository создаёт репозиторий заказов с переданным пулом PostgreSQL.
// За закрытие пула отвечает вызывающий код.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
