# Technical Documentation

## System Architecture

### Overview
The system follows a microservices architecture with event-driven communication, implemented using:
- Go for services
- PostgreSQL for data persistence
- RabbitMQ for event messaging
- Traefik as API Gateway
- Docker for containerization

### Planned Architecture Enhancements
- Circuit breaker pattern for fault tolerance
- Exponential backoff with jitter for retries
- Dead Letter Queue for failed messages
- Comprehensive monitoring system
- Advanced testing infrastructure

### Architecture Diagram

#### Current Architecture
```
┌─────────────┐     ┌─────────────┐
│    Client   │     │    Client   │
└──────┬──────┘     └──────┬──────┘
       │                   │
       ▼                   ▼
┌─────────────────────────────────┐
│         API Gateway (Traefik)   │
│         Port: 8088              │
└─────────────────────────────────┘
       │                   │
       ▼                   ▼
┌─────────────┐     ┌─────────────┐
│   Account   │     │ Transaction │
│   Service   │     │   Service   │
│  Port: 8080 │     │ Port: 8081  │
└──────┬──────┘     └──────┬──────┘
       │                   │
       ▼                   ▼
┌─────────────────────────────────┐
│            RabbitMQ             │
│         Port: 5672              │
│    Management UI: 15672         │
└─────────────────────────────────┘
       │                   │
       ▼                   ▼
┌─────────────┐     ┌─────────────┐
│ PostgreSQL  │     │ PostgreSQL  │
│  Accounts   │     │Transactions │
│  Database   │     │  Database   │
└─────────────┘     └─────────────┘
```

#### Service Structure

1. **Account Service** (`account-service/`)
   ```
   account-service/
   ├── cmd/
   │   └── main.go
   ├── internal/
   │   ├── domain/          # Domain models and interfaces
   │   ├── application/     # Business logic
   │   ├── infrastructure/  # External services (DB, RabbitMQ)
   │   └── interfaces/      # API handlers
   ├── docs/               # API documentation
   ├── go.mod
   └── go.sum
   ```

2. **Transaction Service** (`transaction-service/`)
   ```
   transaction-service/
   ├── cmd/
   │   └── main.go
   ├── internal/
   │   ├── domain/          # Domain models and interfaces
   │   ├── application/     # Business logic
   │   ├── infrastructure/  # External services (DB, RabbitMQ)
   │   └── interfaces/      # API handlers
   ├── docs/               # API documentation
   ├── go.mod
   └── go.sum
   ```

#### Current Components
1. **API Gateway (Traefik)**
   - Port: 8088 (HTTP)
   - Port: 8082 (Dashboard)
   - Request routing
   - Load balancing
   - Basic error handling

2. **Account Service**
   - Port: 8080
   - Account management
   - Balance updates
   - Basic error handling
   - RabbitMQ event publishing

3. **Transaction Service**
   - Port: 8081
   - Transaction processing
   - Status updates
   - Basic error handling
   - RabbitMQ event publishing

4. **RabbitMQ**
   - Port: 5672 (AMQP)
   - Port: 15672 (Management UI)
   - Message broker
   - Event publishing
   - Basic queue management

5. **PostgreSQL**
   - Port: 5432
   - Two separate databases:
     - accounts
     - transactions
   - Data persistence
   - Transaction management
   - Basic data consistency

#### Planned Components
1. **Circuit Breaker**
   - Failure detection
   - Automatic circuit breaking
   - Graceful degradation
   - Automatic recovery

2. **Retry Handler**
   - Exponential backoff
   - Jitter implementation
   - Retry count tracking
   - Error classification

3. **Dead Letter Queue**
   - Failed message handling
   - Retry scheduling
   - Error tracking
   - Alert generation

4. **Monitoring System**
   - Metrics collection
   - Performance monitoring
   - Error tracking
   - Alert management

5. **Enhanced Database**
   - Retry tracking
   - Error logging
   - Audit trails
   - Performance optimization

## Service Details

### Account Service

#### Current Domain Model
```go
type Account struct {
    ID      AccountID
    Balance string
}

type AccountRepository interface {
    Create(ctx context.Context, account *Account) error
    GetByID(ctx context.Context, id AccountID) (*Account, error)
    Update(ctx context.Context, account *Account) error
}
```

#### Planned Domain Model Extensions
```go
type CircuitBreaker struct {
    failures     int
    threshold    int
    resetTimeout time.Duration
    lastFailure  time.Time
    mu           sync.Mutex
}

type RetryConfig struct {
    MaxRetries        int           `json:"max_retries"`
    InitialBackoff    time.Duration `json:"initial_backoff"`
    MaxBackoff        time.Duration `json:"max_backoff"`
    BackoffMultiplier float64       `json:"backoff_multiplier"`
    JitterFactor      float64       `json:"jitter_factor"`
}
```

#### Events
- `account.created`: Published when a new account is created
- `account.updated`: Published when account balance changes

### Transaction Service

#### Domain Model
```go
type Transaction struct {
    ID                  TransactionID
    SourceAccountID     AccountID
    DestinationAccountID AccountID
    Amount              string
    Status              TransactionStatus
    RetryCount          int
    LastError          string
    LastAttempt        time.Time
    NextAttempt        time.Time
}

type TransactionRepository interface {
    Create(ctx context.Context, transaction *Transaction) error
    GetByID(ctx context.Context, id TransactionID) (*Transaction, error)
    Update(ctx context.Context, transaction *Transaction) error
}

type ErrorType int

const (
    TemporaryError ErrorType = iota
    PermanentError
    UnknownError
)
```

#### Events
- `transaction.submitted`: Published when a transaction is initiated
- `transaction.completed`: Published when transaction succeeds
- `transaction.failed`: Published when transaction fails

## Database Schema

### Accounts Table
```sql
CREATE TABLE accounts (
    id BIGINT PRIMARY KEY,
    balance TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_accounts_id ON accounts(id);
```

### Transactions Table
```sql
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    source_account_id BIGINT NOT NULL,
    destination_account_id BIGINT NOT NULL,
    amount TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'complete', 'failed', 'rollback')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_transactions_id ON transactions(id);
CREATE INDEX idx_transactions_source_account ON transactions(source_account_id);
CREATE INDEX idx_transactions_destination_account ON transactions(destination_account_id);
CREATE INDEX idx_transactions_status ON transactions(status);

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

### Planned Schema Enhancements
```sql
-- Planned additions to transactions table
ALTER TABLE transactions
    ADD COLUMN retry_count INT DEFAULT 0,
    ADD COLUMN last_error TEXT,
    ADD COLUMN last_attempt TIMESTAMP WITH TIME ZONE,
    ADD COLUMN next_attempt TIMESTAMP WITH TIME ZONE,
    ADD CONSTRAINT different_accounts CHECK (source_account_id != destination_account_id);
```

## Message Broker Configuration

### RabbitMQ Exchanges
- `account_events`: Topic exchange for account-related events
- `transaction_events`: Topic exchange for transaction-related events

### Routing Keys
- Account Events:
  - `account.created`
  - `account.updated`
- Transaction Events:
  - `transaction.submitted`
  - `transaction.completed`
  - `transaction.failed`

### Dead Letter Queue Configuration
```go
type DLQConfig struct {
    Name           string
    MaxRetries     int
    RetryInterval  time.Duration
    AlertThreshold int
}
```

## API Gateway Configuration

### Traefik Routes
- Account Service: `/api/v1/accounts/*`
- Transaction Service: `/api/v1/transactions/*`

### Middleware
- Request logging
- Error handling
- Service discovery
- Circuit breaker
- Rate limiting

## Error Handling

### Current Implementation
- Basic error handling for common scenarios
- Transaction status tracking
- Event replay capability
- Database transaction rollback
- Error logging

### Planned Error Handling
```go
func classifyError(err error) ErrorType {
    switch {
    case errors.Is(err, ErrNetworkTimeout):
        return TemporaryError
    case errors.Is(err, ErrInvalidAccount):
        return PermanentError
    default:
        return UnknownError
    }
}
```

## Testing Strategy

### Current Implementation
- Basic manual testing
- Simple error logging
- Basic transaction flow testing

### Planned Testing Infrastructure
1. **Unit Tests**
   - Domain logic testing
   - Service layer testing
   - Repository testing
   - Circuit breaker testing
   - Retry mechanism testing

2. **Integration Tests**
   - Service communication testing
   - Database integration testing
   - Message broker integration testing
   - API endpoint testing

3. **End-to-End Tests**
   - Complete transaction flow testing
   - Error scenario testing
   - Performance testing
   - Load testing

## Monitoring and Observability

### Current Implementation
- Basic logging
- Simple error tracking
- Basic transaction status monitoring

### Planned Monitoring
- Request latency tracking
- Error rate monitoring
- Message queue length monitoring
- Database connection pool status
- Circuit breaker status
- Retry attempt tracking
- Processing time metrics
- DLQ monitoring

## Security Considerations

### Current Implementation
- No authentication/authorization (as per requirements)
- Input validation for all API endpoints
- SQL injection prevention using parameterized queries
- Circuit breaker protection
- Rate limiting

### Future Improvements
- JWT-based authentication
- Rate limiting
- API key management
- SSL/TLS encryption
- Audit logging
- Security monitoring

## Deployment

### Docker Configuration
- Multi-stage builds for smaller images
- Environment variable configuration
- Health checks for all services
- Volume mounts for data persistence
- Circuit breaker configuration
- Retry mechanism settings

### Scaling Considerations
- Stateless services for horizontal scaling
- Database connection pooling
- Message queue partitioning
- Load balancing through API Gateway
- Circuit breaker thresholds
- Retry mechanism tuning

## Development Guidelines

### Code Organization
```
service/
├── cmd/
│   └── main.go
├── internal/
│   ├── domain/
│   ├── application/
│   ├── infrastructure/
│   └── interfaces/
├── db/
│   └── init.sql
└── Dockerfile
```

### Testing Strategy
- Unit tests for domain logic
- Integration tests for repositories
- End-to-end tests for API endpoints
- Message broker integration tests
- Circuit breaker tests
- Retry mechanism tests
- DLQ handling tests

## Performance Considerations

### Database
- Indexed queries
- Connection pooling
- Prepared statements
- Transaction batching
- Retry count tracking
- Error logging

### Message Broker
- Message persistence
- Publisher confirms
- Consumer prefetch
- Queue length monitoring
- DLQ management
- Circuit breaker integration

### API Gateway
- Response caching
- Request compression
- Connection pooling
- Circuit breaking
- Rate limiting
- Error handling 