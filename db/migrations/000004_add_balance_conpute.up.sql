CREATE FUNCTION get_account_balance(account_id UUID) RETURNS BIGINT AS $$
    SELECT
        (
            SELECT COALESCE(SUM(amount), 0)
            FROM transaction
            WHERE credit_account_id = get_account_balance.account_id
        )       -- Credited to account
            -   -- Minus
        (       -- Debited from account
            SELECT COALESCE(SUM(amount), 0)
            FROM transaction
            WHERE debit_account_id = get_account_balance.account_id
        )
$$ LANGUAGE SQL;
