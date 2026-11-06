package models

type Product struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	Available  int    `json:"available"`
}
