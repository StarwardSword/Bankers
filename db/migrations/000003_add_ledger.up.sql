CREATE TABLE IF NOT EXISTS transaction (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid()
    ,created_at TIMESTAMPTZ DEFAULT now()
    ,debit_account_id UUID REFERENCES accounts(id)
    ,credit_account_id UUID NOT NULL REFERENCES accounts(id)
    ,amount BIGINT NOT NULL
);
