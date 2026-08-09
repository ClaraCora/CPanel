ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_reset_strategy_check;

ALTER TABLE plans
    ADD CONSTRAINT plans_reset_strategy_check
    CHECK (reset_strategy IN ('calendar_month', 'never'));
