package store

// runtimeEligibleUsersCTE is the single source of truth for runtime access to
// nodes. A disabled access group revokes access immediately. Plan status is
// intentionally not checked: disabling a plan only prevents new assignments
// and does not interrupt users who were already assigned to it.
const entitledUsersCTE = `
WITH entitled_users AS (
	SELECT u.id,u.agent_id,u.uuid,u.name,u.email,u.portal_login,u.role,u.status,
	       COALESCE(u.portal_password_hash,'') AS portal_password_hash,
	       u.expires_at,u.traffic_used_bytes,u.traffic_reset_at,
	       p.name AS plan_name,
	       COALESCE(u.traffic_limit_override_bytes,p.traffic_limit_bytes,0) AS traffic_limit_bytes,
	       COALESCE(u.speed_limit_override_mbps,p.speed_limit_mbps,0) AS speed_limit_mbps,
	       COALESCE(u.device_limit_override,p.device_limit,0) AS device_limit,
	       g.id AS access_group_id,u.subscription_token_hash
	FROM users u
	LEFT JOIN plans p ON p.id=u.plan_id
	JOIN access_groups g
	  ON g.id=COALESCE(u.access_group_override_id,p.access_group_id)
	 AND g.status='active'
	WHERE u.status='active'
	  AND (u.expires_at IS NULL OR u.expires_at > now())
)
`

// Quota only gates new/runtime access. Traffic intake deliberately uses the
// entitlement relation above, so a final in-flight report after quota revocation
// is still charged instead of being silently discarded.
const runtimeEligibleUsersCTE = entitledUsersCTE + `, runtime_eligible_users AS (
	SELECT * FROM entitled_users
	WHERE traffic_limit_bytes=0 OR traffic_used_bytes < traffic_limit_bytes
)
`

// Final traffic can arrive after quota, expiry, pause or group revocation. It is
// accepted only for an existing account currently bound to this node's group;
// this accounting relation never grants runtime access or portal access.
const trafficAccountableUsersCTE = `WITH traffic_accountable_users AS (
	SELECT u.id,u.agent_id,u.traffic_used_bytes,
	       COALESCE(u.traffic_limit_override_bytes,p.traffic_limit_bytes,0) AS traffic_limit_bytes,
	       COALESCE(u.access_group_override_id,p.access_group_id) AS access_group_id
	FROM users u LEFT JOIN plans p ON p.id=u.plan_id
	WHERE u.status <> 'archived'
)
`
