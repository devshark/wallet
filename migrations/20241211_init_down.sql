-- transactions
DROP TABLE IF EXISTS public."transactions";

-- accounts
DROP TABLE IF EXISTS public."accounts";

-- -- Create a trigger for accounts table
DROP TRIGGER IF EXISTS update_accounts_modified_time ON accounts;

-- Create a trigger for transactions table
DROP TRIGGER IF EXISTS update_transactions_modified_time ON transactions;

-- Drop the function
DROP FUNCTION IF EXISTS update_modified_column;

-- Drop the type
DROP TYPE IF EXISTS "DebitCredit";

-- Disable the uuid-ossp extension
DROP EXTENSION IF EXISTS "uuid-ossp";

