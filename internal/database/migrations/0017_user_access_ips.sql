CREATE TABLE user_access_ips (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address INET NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    last_node_id TEXT REFERENCES nodes(id) ON DELETE SET NULL,
    last_node_name TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(user_id, ip_address),
    CHECK(last_seen_at >= first_seen_at)
);

CREATE INDEX user_access_ips_last_seen_idx
    ON user_access_ips(last_seen_at DESC);

WITH aggregated AS (
    SELECT
        d.user_id,
        d.ip_address,
        min(d.first_seen_at) AS first_seen_at,
        max(d.last_seen_at) AS last_seen_at,
        (array_agg(d.node_id ORDER BY d.last_seen_at DESC, d.node_id))[1] AS last_node_id,
        (array_agg(n.name ORDER BY d.last_seen_at DESC, d.node_id))[1] AS last_node_name
    FROM user_devices d
    JOIN nodes n ON n.id=d.node_id
    GROUP BY d.user_id,d.ip_address
), ranked AS (
    SELECT aggregated.*,
           row_number() OVER (PARTITION BY user_id ORDER BY last_seen_at DESC, ip_address DESC) AS position
    FROM aggregated
)
INSERT INTO user_access_ips(user_id,ip_address,first_seen_at,last_seen_at,last_node_id,last_node_name)
SELECT user_id,ip_address,first_seen_at,last_seen_at,last_node_id,last_node_name
FROM ranked
WHERE position <= 10;
