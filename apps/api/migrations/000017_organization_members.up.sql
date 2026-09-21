-- Real organization membership.
--
-- Membership was a single users.org_id column, which AssignUserToOrg overwrote.
-- A user therefore belonged to exactly one organization at a time: the
-- dashboard's organization switcher kept a list the backend never recognised,
-- and a "role" was global to the user rather than scoped to an organization.
CREATE TABLE IF NOT EXISTS organization_members (
    org_id     UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(32) NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, user_id),
    CONSTRAINT organization_members_role_check CHECK (role IN ('owner', 'admin', 'member'))
);

CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members (user_id);

-- Backfill from the existing single-org column so nobody loses access.
-- users.org_id is kept as the user's default organization.
INSERT INTO organization_members (org_id, user_id, role)
SELECT u.org_id, u.id, CASE WHEN u.role IN ('owner', 'admin') THEN u.role ELSE 'member' END
FROM users u
WHERE u.org_id IS NOT NULL
ON CONFLICT (org_id, user_id) DO NOTHING;
