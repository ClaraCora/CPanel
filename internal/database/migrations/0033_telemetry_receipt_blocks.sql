-- A bit is permanent proof of one accepted sequence number for one machine
-- and Agent process. Out-of-order numbers leave holes that can be filled later.
CREATE TABLE telemetry_receipt_blocks (
    machine_id TEXT NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    stream_id UUID NOT NULL,
    block_number BIGINT NOT NULL,
    received BIT(1024) NOT NULL,
    PRIMARY KEY (machine_id, stream_id, block_number)
);

-- NULL denotes a legacy opaque key. Keep one parser for both panel versions.
CREATE FUNCTION try_claim_telemetry_sequence(p_machine TEXT, p_key TEXT)
RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    sequence_text TEXT;
    sequence_index BIGINT;
    bit_index INTEGER;
BEGIN
    IF p_key COLLATE "C" !~ '^t2-[0-9a-f]{32}-[1-9][0-9]{0,18}$' THEN
        RETURN NULL;
    END IF;
    sequence_text := substr(p_key, 37);
    IF length(sequence_text) = 19 AND sequence_text COLLATE "C" > '9223372036854775807' THEN
        RETURN NULL;
    END IF;
    -- New Agents may already have reported to an older panel before migration.
    -- Its primary key answers this lookup without rewriting existing receipts.
    IF EXISTS (SELECT 1 FROM telemetry_batches WHERE machine_id=p_machine AND idempotency_key=p_key) THEN
        RETURN FALSE;
    END IF;
    sequence_index := sequence_text::BIGINT - 1;
    bit_index := (sequence_index % 1024)::INTEGER;
    INSERT INTO telemetry_receipt_blocks AS b(machine_id, stream_id, block_number, received)
    VALUES (p_machine, substr(p_key, 4, 32)::UUID, sequence_index / 1024, set_bit(B'0'::BIT(1024), bit_index, 1))
    ON CONFLICT (machine_id, stream_id, block_number) DO UPDATE
        SET received=set_bit(b.received, bit_index, 1)
        WHERE get_bit(b.received, bit_index)=0;
    RETURN FOUND;
END;
$$;

CREATE FUNCTION claim_telemetry_batch(p_machine TEXT, p_key TEXT)
RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    accepted BOOLEAN;
BEGIN
    IF left(p_key, 3) = 't2-' THEN
        accepted := try_claim_telemetry_sequence(p_machine, p_key);
        IF accepted IS NOT NULL THEN
            RETURN accepted;
        END IF;
    END IF;
    INSERT INTO telemetry_batches(machine_id, idempotency_key) VALUES (p_machine, p_key)
    ON CONFLICT DO NOTHING;
    RETURN FOUND;
END;
$$;

-- Older binaries still INSERT directly and use the affected-row count to
-- decide whether to apply traffic. Their transaction must acquire the same bit.
CREATE FUNCTION check_legacy_telemetry_insert()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF try_claim_telemetry_sequence(NEW.machine_id, NEW.idempotency_key) IS FALSE THEN
        RETURN NULL;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER check_legacy_telemetry_insert
BEFORE INSERT ON telemetry_batches
FOR EACH ROW WHEN (left(NEW.idempotency_key, 3) = 't2-')
EXECUTE FUNCTION check_legacy_telemetry_insert();
