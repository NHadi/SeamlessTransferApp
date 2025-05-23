package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "internal-transfers/transaction-service/docs"
	"internal-transfers/transaction-service/internal/application"
	"internal-transfers/transaction-service/internal/domain"
	"internal-transfers/transaction-service/internal/infrastructure/messaging"
	"internal-transfers/transaction-service/internal/infrastructure/postgres"
	httpHandler "internal-transfers/transaction-service/internal/interfaces/http"

	"log/slog"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("Starting transaction service", "port", "8081")

	// Initialize database connection
	db, err := postgres.NewDBPool(context.Background())
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize RabbitMQ connection
	broker, err := messaging.NewRabbitMQBroker()
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer broker.Close()

	// Initialize repositories
	transactionRepo := postgres.NewTransactionRepository(db)

	// Initialize services
	transactionService := application.NewTransactionService(transactionRepo, broker)

	// Subscribe to transaction events
	if err := broker.SubscribeToTransactionEvents(context.Background(), func(event domain.TransactionEvent) error {
		switch event.Status {
		case string(domain.TransactionStatusComplete):
			return transactionService.HandleTransactionCompleted(context.Background(), event)
		case string(domain.TransactionStatusFailed):
			return transactionService.HandleTransactionFailed(context.Background(), event)
		default:
			return nil
		}
	}); err != nil {
		logger.Error("Failed to subscribe to transaction events", "error", err)
		os.Exit(1)
	}

	// Initialize handlers
	transactionHandler := httpHandler.NewTransactionHandler(transactionService)

	// Setup router
	r := chi.NewRouter()

	// Swagger
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8081/swagger/doc.json"),
	))

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		httpHandler.RegisterHandlers(r, transactionHandler)
	})

	// Create HTTP server
	server := &http.Server{
		Addr:    ":8081",
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Transaction service ready to accept requests", "port", "8081")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server exited")
}
