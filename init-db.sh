#!/bin/bash
set -e

# Create accounts database
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "CREATE DATABASE accounts;"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "GRANT ALL PRIVILEGES ON DATABASE accounts TO postgres;"

# Create transactions database
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "CREATE DATABASE transactions;"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "GRANT ALL PRIVILEGES ON DATABASE transactions TO postgres;"

# Create accounts table
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "accounts" -c "
    CREATE TABLE IF NOT EXISTS accounts (
        id BIGINT PRIMARY KEY,
        balance TEXT NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "accounts" -c "
    CREATE INDEX IF NOT EXISTS idx_accounts_id ON accounts(id);"

# Create transactions table
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "transactions" -c "
    CREATE TABLE IF NOT EXISTS transactions (
        id SERIAL PRIMARY KEY,
        source_account_id BIGINT NOT NULL,
        destination_account_id BIGINT NOT NULL,
        amount TEXT NOT NULL,
        status TEXT NOT NULL CHECK (status IN ('pending', 'complete', 'failed', 'rollback')),
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );"

# Create indexes for transactions
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "transactions" -c "
    CREATE INDEX IF NOT EXISTS idx_transactions_id ON transactions(id);
    CREATE INDEX IF NOT EXISTS idx_transactions_source_account ON transactions(source_account_id);
    CREATE INDEX IF NOT EXISTS idx_transactions_destination_account ON transactions(destination_account_id);
    CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);"

# Create trigger function and trigger
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "transactions" -c "
    CREATE OR REPLACE FUNCTION update_updated_at_column()
    RETURNS TRIGGER AS \$\$
    BEGIN
        NEW.updated_at = CURRENT_TIMESTAMP;
        RETURN NEW;
    END;
    \$\$ language 'plpgsql';"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "transactions" -c "
    DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
    CREATE TRIGGER update_transactions_updated_at
        BEFORE UPDATE ON transactions
        FOR EACH ROW
        EXECUTE FUNCTION update_updated_at_column();" 