-- This is based on Netbox's schema, mostly the ipam and tenancy
-- tables.

CREATE TABLE ipam_ipaddress (
    id integer NOT NULL,
    created date default now() NOT NULL,
    last_updated timestamp with time zone,
    address cidr NOT NULL,
    description character varying(200) NOT NULL,
    interface_id integer,
    nat_inside_id integer,
    vrf_id integer,
    tenant_id integer,
    status character varying(50) NOT NULL,
    role character varying(50) NOT NULL,
    dns_name character varying(255) NOT NULL
);
CREATE SEQUENCE ipam_ipaddress_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE ipam_ipaddress_id_seq OWNED BY ipam_ipaddress.id;
CREATE TRIGGER ipam_ipaddress_last_updated
	BEFORE UPDATE ON ipam_ipaddress
	FOR EACH ROW
	EXECUTE PROCEDURE moddatetime (last_updated);


CREATE TABLE tenancy_tenant (
    id integer NOT NULL,
    created date default now() NOT NULL,
    last_updated timestamp with time zone,
    name character varying(30) NOT NULL,
    slug character varying(50) NOT NULL,
    description character varying(200) NOT NULL,
    comments text NOT NULL,
    group_id integer
);
CREATE SEQUENCE tenancy_tenant_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE tenancy_tenant_id_seq OWNED BY tenancy_tenant.id;
CREATE TRIGGER tenancy_tenant_last_updated
	BEFORE UPDATE ON tenancy_tenant
	FOR EACH ROW
	EXECUTE PROCEDURE moddatetime (last_updated);


ALTER TABLE ONLY ipam_ipaddress ALTER COLUMN id SET DEFAULT nextval('ipam_ipaddress_id_seq'::regclass);
ALTER TABLE ONLY tenancy_tenant ALTER COLUMN id SET DEFAULT nextval('tenancy_tenant_id_seq'::regclass);


ALTER TABLE ONLY ipam_ipaddress
    ADD CONSTRAINT ipam_ipaddress_nat_inside_id_key UNIQUE (nat_inside_id);
ALTER TABLE ONLY ipam_ipaddress
    ADD CONSTRAINT ipam_ipaddress_pkey PRIMARY KEY (id);
ALTER TABLE ONLY tenancy_tenant
    ADD CONSTRAINT tenancy_tenant_name_key UNIQUE (name);
ALTER TABLE ONLY tenancy_tenant
    ADD CONSTRAINT tenancy_tenant_pkey PRIMARY KEY (id);
ALTER TABLE ONLY tenancy_tenant
    ADD CONSTRAINT tenancy_tenant_slug_key UNIQUE (slug);


CREATE INDEX ipam_ipaddress_interface_id_91e71d9d ON ipam_ipaddress USING btree (interface_id);
CREATE INDEX ipam_ipaddress_tenant_id_ac55acfd ON ipam_ipaddress USING btree (tenant_id);
CREATE INDEX ipam_ipaddress_vrf_id_51fcc59b ON ipam_ipaddress USING btree (vrf_id);
CREATE INDEX tenancy_tenant_group_id_7daef6f4 ON tenancy_tenant USING btree (group_id);
CREATE INDEX tenancy_tenant_name_f6e5b2f5_like ON tenancy_tenant USING btree (name varchar_pattern_ops);
CREATE INDEX tenancy_tenant_slug_0716575e_like ON tenancy_tenant USING btree (slug varchar_pattern_ops);


ALTER TABLE ONLY ipam_ipaddress
    ADD CONSTRAINT ipam_ipaddress_nat_inside_id_a45fb7c5_fk_ipam_ipaddress_id FOREIGN KEY (nat_inside_id) REFERENCES ipam_ipaddress(id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE ONLY ipam_ipaddress
    ADD CONSTRAINT ipam_ipaddress_tenant_id_ac55acfd_fk_tenancy_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenancy_tenant(id) DEFERRABLE INITIALLY DEFERRED;
