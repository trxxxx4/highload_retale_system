package models

type StockItem struct {
	SKU       string `json:"sku"`
	Available int    `json:"available"`
}

type Stock struct {
	SKU       string `json:"sku"`
	OnHand    int    `json:"on_hand"`
	Reserved  int    `json:"reserved"`
	Available int    `json:"available"`
}

type ResponseStock struct {
	Items     interface{} `json:"items"`
	NextAfter string      `json:"next_after"`
}
