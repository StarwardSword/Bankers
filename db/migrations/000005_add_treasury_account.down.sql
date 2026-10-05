WITH treasury AS (
    SELECT
        accounts.id as account_id
        ,users.id as user_id
    FROM users
    JOIN accounts ON accounts.holder_id = users.id
    WHERE users.username = 'treasury'
),
emission AS (
    DELETE FROM transaction
    USING treasury
    WHERE transaction.debit_account_id IS NULL
        AND transaction.credit_account_id = treasury.account_id
),
treasury_account AS (
    DELETE FROM accounts
    USING treasury
    WHERE accounts.id = treasury.account_id
)
DELETE FROM users
USING treasury
WHERE users.id = treasury.user_id;
