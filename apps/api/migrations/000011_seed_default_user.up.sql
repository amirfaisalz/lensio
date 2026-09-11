INSERT INTO users (id, org_id, email, full_name, role)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000001',
    'dev@nusaid.dev',
    'NusaID Lead Developer',
    'owner'
)
ON CONFLICT (email) DO NOTHING;
