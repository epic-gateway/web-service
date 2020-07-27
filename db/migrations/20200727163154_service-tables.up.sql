CREATE TABLE groups (
    id uuid DEFAULT uuid_generate_v4(),
    name VARCHAR NOT NULL,
    created timestamp with time zone default now() NOT NULL,
    updated timestamp with time zone,
    PRIMARY KEY (id)
);
CREATE TRIGGER groups_updated
	BEFORE UPDATE ON groups
	FOR EACH ROW
	EXECUTE PROCEDURE moddatetime (updated);

CREATE TABLE services (
    id uuid DEFAULT uuid_generate_v4(),
    group_id uuid REFERENCES groups(id),
    name VARCHAR NOT NULL,
    address cidr NOT NULL,
    created timestamp with time zone default now() NOT NULL,
    updated timestamp with time zone,
    PRIMARY KEY (id)
);
CREATE TRIGGER services_updated
	BEFORE UPDATE ON services
	FOR EACH ROW
	EXECUTE PROCEDURE moddatetime (updated);

CREATE TABLE endpoints (
    id uuid DEFAULT uuid_generate_v4(),
    service_id uuid REFERENCES services(id),
    address cidr NOT NULL,
    port integer NOT NULL,
    created timestamp with time zone default now() NOT NULL,
    updated timestamp with time zone,
    PRIMARY KEY (id)
);
CREATE TRIGGER endpoints_updated
	BEFORE UPDATE ON endpoints
	FOR EACH ROW
	EXECUTE PROCEDURE moddatetime (updated);
