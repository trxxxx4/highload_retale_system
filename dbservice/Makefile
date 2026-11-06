include .env
export

run:
	go run main.go

migrate-up:
	migrate -path migrations -database ${CONNECT} up

migrate-down:
	migrate -path migrations -database ${CONNECT} down