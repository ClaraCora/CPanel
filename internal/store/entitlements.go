package store

// runtimeEligibleUsersCTE is the single source of truth for runtime access to
// nodes. A disabled access group revokes access immediately. Plan status is
// intentionally not checked: disabling a plan only prevents new assignments
// and does not interrupt users who were already assigned to it.
const runtimeEligibleUsersCTE = `
WITH runtime_eligible_users AS (
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
