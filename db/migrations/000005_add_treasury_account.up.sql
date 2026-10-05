WITH params AS (
    SELECT 
        1_000::bigint as all_currency
        ,'treasury'::varchar as treasury_user_name 
        ,'piP7M+AcnQLh78UlnSBB68KZiRuF6stZo/3O3Ftx6VoKQr849KK2lvVZqta5Quswv/Ck3JUibjFxXYbSVy+gbA=='::varchar as treasury_user_hash 
), 
new_user AS (
    INSERT INTO users(username, password, role) 
    SELECT treasury_user_name as username, treasury_user_hash as password, 1 as role FROM params
    RETURNING id
),
new_account AS (
    INSERT INTO accounts(holder_id) 
    SELECT new_user.id as holder_id FROM new_user
    RETURNING id
)
-- Emission has no debit account
INSERT INTO transaction(credit_account_id, amount)
SELECT new_account.id, all_currency FROM new_account, params;
