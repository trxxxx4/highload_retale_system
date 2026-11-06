package repository

import (
	"context"
	"db/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StockRepository struct {
	Pool *pgxpool.Pool
}

func NewStockRepository(pool *pgxpool.Pool) *StockRepository {
	return &StockRepository{
		Pool: pool,
	}
}

func (repo *StockRepository) GetAfter(
	ctx context.Context,
	after string,
	limit int,
) ([]models.StockItem, error) {
	query := `
	SELECT 
		p.sku,
		s.on_hand - s.reserved AS available
	FROM products AS p
	JOIN stock AS s ON s.product_id = p.id
	WHERE p.sku > $1
	ORDER BY p.sku
	LIMIT $2;
	`
	rows, err := repo.Pool.Query(
		ctx, query, after, limit,
	)
	if err != nil {
		return nil, err
	}
	stock := make([]models.StockItem, 0)
	defer rows.Close()
	for rows.Next() {
		var stockItem models.StockItem

		err := rows.Scan(
			&stockItem.SKU,
			&stockItem.Available,
		)
		if err != nil {
			return nil, err
		}

		stock = append(stock, stockItem)

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stock, nil
}

func (repo *StockRepository) GetBySKU(
	ctx context.Context,
	sku string,
) (*models.Stock, error) {
	query := `
    SELECT
        p.sku,
        s.on_hand,
        s.reserved,
        s.on_hand - s.reserved AS available
    FROM products AS p
    JOIN stock AS s ON s.product_id = p.id
    WHERE p.sku = $1;
    `

	var stockItem models.Stock

	err := repo.Pool.QueryRow(
		ctx, query, sku,
	).Scan(
		&stockItem.SKU,
		&stockItem.OnHand,
		&stockItem.Reserved,
		&stockItem.Available,
	)
	if err != nil {
		return nil, err
	}

	return &stockItem, nil
}
