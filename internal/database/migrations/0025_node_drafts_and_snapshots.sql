-- Keep editable drafts away from the rows used by Agents and subscriptions.
CREATE TABLE node_drafts (
    node_id TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    document JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Old revisions only contain protocol settings. NULL explicitly identifies an
-- incomplete historical record; it must never be presented as a full rollback.
ALTER TABLE node_revisions ADD COLUMN snapshot JSONB;
