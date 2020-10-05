INSERT INTO tenancy_tenant
(id, name, slug, description, comments)
VALUES
(1, 'IPAM PureLB Customer EXP', 'ipam-purelb-customer-exp', 'For experimentation with PureLB as an IPAM client', '')
ON CONFLICT DO NOTHING;

INSERT INTO ipam_ipaddress
(id, address, description, tenant_id, status, role, dns_name)
VALUES
(1, '172.30.254.2/32', 'seeded', 1, 'reserved', '', ''),
(2, '172.30.254.3/32', 'seeded', 1, 'reserved', '', ''),
(3, '172.30.254.4/32', 'seeded', 1, 'reserved', '', ''),
(4, '172.30.254.5/32', 'seeded', 1, 'reserved', '', ''),
(5, '172.30.254.6/32', 'seeded', 1, 'reserved', '', '')
ON CONFLICT DO NOTHING;
