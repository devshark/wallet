-- Enable the uuid-ossp extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- DebitCredit
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'debitcredit') THEN
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

-- -- accounts
-- CREATE TABLE IF NOT EXISTS public."accounts" (
--     "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
--     "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
--     "updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

--     -- This creates a unique index on name and currency columns
--     CONSTRAINT unique_account_currency UNIQUE (name, currency)
-- );

-- -- Create a trigger for accounts table
-- CREATE OR REPLACE TRIGGER update_accounts_modified_time
-- BEFORE UPDATE ON accounts
-- FOR EACH ROW
-- EXECUTE FUNCTION update_modified_column();

-- transactions
CREATE TABLE IF NOT EXISTS public."transactions" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "account_id" VARCHAR(255) NOT NULL,
    "balance" NUMERIC NOT NULL,
    "currency" VARCHAR(3) NOT NULL,
    "group_id" VARCHAR(50) NOT NULL, -- Used for idempotency
    "description" VARCHAR(255),
    "debit_credit" "DebitCredit" NOT NULL,
    "amount" NUMERIC NOT NULL,
    -- "account_id" UUID NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Foreign key
    -- CONSTRAINT "transactions_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "accounts"("id") ON DELETE RESTRICT ON UPDATE CASCADE,

    -- This creates a unique index on group_id and debit_credit columns.
    -- This ensures idempotency of ledger entries.
    CONSTRAINT unique_transaction_entry UNIQUE (group_id, debit_credit, account_id)

    -- This creates a unique index on name and currency columns
    -- CONSTRAINT unique_account_currency UNIQUE (account_id, currency)
);

-- Create a trigger for transactions table
CREATE OR REPLACE TRIGGER update_transactions_modified_time
BEFORE UPDATE ON transactions
FOR EACH ROW
EXECUTE FUNCTION update_modified_column();
