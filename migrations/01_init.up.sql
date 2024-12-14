-- Enable the uuid-ossp extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- DebitCredit
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'DebitCredit') THEN
        CREATE TYPE "DebitCredit" AS ENUM ('DEBIT', 'CREDIT');
    END IF;
END
$$;

-- Create a function to update the timestamp
CREATE OR REPLACE FUNCTION update_modified_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- accounts
CREATE TABLE IF NOT EXISTS public."accounts" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(), -- acts as a surrogate key only
    "user_id" VARCHAR(255) NOT NULL,
    "currency" VARCHAR(10) NOT NULL,
    "balance" NUMERIC NOT NULL DEFAULT 0,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- the natural key is user_id + currency
    CONSTRAINT unique_user_currency UNIQUE (user_id, currency)
);

-- Create a trigger for accounts table
CREATE OR REPLACE TRIGGER update_accounts_modified_time
BEFORE UPDATE ON public."accounts"
FOR EACH ROW
EXECUTE FUNCTION update_modified_column();

-- transactions
CREATE TABLE IF NOT EXISTS public."transactions" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "account_id" UUID NOT NULL,
    -- "currency" VARCHAR(3) NOT NULL,
    "amount" NUMERIC NOT NULL,
    "debit_credit" "DebitCredit" NOT NULL,
    "group_id" VARCHAR(50) NOT NULL, -- Used for idempotency
    "description" VARCHAR(255),
    -- "balance" NUMERIC,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- This creates a unique index on group_id and debit_credit columns.
    -- This ensures idempotency of ledger entries.
    CONSTRAINT unique_transaction_entry UNIQUE (group_id, debit_credit, account_id),

    -- foreign key
    CONSTRAINT account_fk FOREIGN KEY (account_id) REFERENCES accounts(id)
);

CREATE INDEX IF NOT EXISTS group_id_idx ON public."transactions" (group_id);
-- CREATE INDEX IF NOT EXISTS account_currency_idx ON public."transactions" (account_id, currency);

-- Create a trigger for transactions table
CREATE OR REPLACE TRIGGER update_transactions_modified_time
BEFORE UPDATE ON public."transactions"
FOR EACH ROW
EXECUTE FUNCTION update_modified_column();
