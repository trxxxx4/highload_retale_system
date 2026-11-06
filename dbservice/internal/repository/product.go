package repository

import (
	"context"
	"db/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	Pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{
		Pool: pool,
	}
}

func (repo *ProductRepository) GetAll(
	ctx context.Context,
	limit int,
	offset int,
) ([]models.Product, error) {
	query := `
	SELECT
		p.sku,
		p.name,
		p.price_cents,
		s.on_hand - s.reserved AS available
	FROM products AS p
	JOIN stock AS s ON s.product_id = p.id
	ORDER BY p.id
	LIMIT $1 OFFSET $2;
	`

	rows, err := repo.Pool.Query(
		ctx,
		query,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	products := make([]models.Product, 0)

	for rows.Next() {
		var product models.Product

		err := rows.Scan(
			&product.SKU,
			&product.Name,
			&product.PriceCents,
			&product.Available,
		)

		if err != nil {
			return nil, err
		}

		products = append(products, product)

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}
