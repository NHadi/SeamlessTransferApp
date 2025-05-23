package main

import (
	"context"
	"net/http"
	"os"

	_ "internal-transfers/account-service/docs"
	"internal-transfers/account-service/internal/application"
	"internal-transfers/account-service/internal/infrastructure/messaging"
	"internal-transfers/account-service/internal/infrastructure/postgres"
	httpHandler "internal-transfers/account-service/internal/interfaces/http"

	"log/slog"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("Starting account service", "port", "8080")

	ctx := context.Background()

	// Initialize database
	dbPool, err := postgres.NewDBPool(ctx)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Initialize RabbitMQ
	broker, err := messaging.NewRabbitMQBroker()
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer broker.Close()

	// Initialize repositories and services
	accountRepo := postgres.NewAccountRepository(dbPool)
	accountService := application.NewAccountService(accountRepo, broker)
	accountHandler := httpHandler.NewAccountHandler(accountService)

	// Subscribe to transaction events
	if err := broker.SubscribeToTransactionEvents(ctx, accountService.HandleTransactionSubmitted); err != nil {
		logger.Error("Failed to subscribe to transaction events", "error", err)
		os.Exit(1)
	}

	// Setup router
	r := chi.NewRouter()

	// Swagger
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		httpHandler.RegisterHandlers(r, accountHandler)
	})

	logger.Info("Account service ready to accept requests", "port", "8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
