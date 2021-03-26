
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

CREATE SCHEMA cothority;
ALTER SCHEMA cothority OWNER TO bypros;

SET default_tablespace = '';
SET default_table_access_method = heap;

--
-- Migration / Versioning
--

CREATE TABLE cothority.version (
    version_id integer
        PRIMARY KEY
        GENERATED ALWAYS AS IDENTITY,

    database_version integer NOT NULL,
    ts TIMESTAMP NOT NULL
);
ALTER TABLE cothority.version OWNER TO bypros;

--
-- Block
--

CREATE TABLE cothority.block (
    block_id integer
        PRIMARY KEY
        GENERATED ALWAYS AS IDENTITY,

    hash bytea NOT NULL
);
ALTER TABLE cothority.block OWNER TO bypros;

--
-- Transaction
--

CREATE TABLE cothority.transaction (
    transaction_id integer
        PRIMARY KEY
        GENERATED ALWAYS AS IDENTITY,

    block_id integer NOT NULL
        REFERENCES cothority.block(block_id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    accepted boolean NOT NULL
);
ALTER TABLE cothority.transaction OWNER TO bypros;

--
-- Instruction
--

CREATE TABLE cothority."instructionType" (
    type_id integer
        PRIMARY KEY
        GENERATED ALWAYS AS IDENTITY,

    name character varying(10) NOT NULL
);

COPY cothority."instructionType" (type_id, name) FROM stdin;
1	Invalid
2	Spawn
3	Invoke
4	Delete
\.

ALTER TABLE cothority."instructionType" OWNER TO bypros;

CREATE TABLE cothority.instruction (
    instruction_id integer
        PRIMARY KEY
        GENERATED ALWAYS AS IDENTITY,

    transaction_id integer NOT NULL
        REFERENCES cothority.transaction(transaction_id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    type_id integer NOT NULL
        REFERENCES cothority."instructionType"(type_id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    action character varying NOT NULL,
    instance_iid bytea NOT NULL,
    contract_iid bytea NOT NULL,
    contract_name character varying NOT NULL
);

ALTER TABLE cothority.instruction OWNER TO bypros;

--
-- Argument
--

CREATE TABLE cothority.argument (
    argument_id integer
        PRIMARY KEY
        GENERATED ALWAYS AS IDENTITY,

    instruction_id integer NOT NULL
        REFERENCES cothority.instruction(instruction_id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    name character varying NOT NULL,
    value bytea
);

ALTER TABLE cothority.argument OWNER TO bypros;

--
-- Signer
--

CREATE TABLE cothority.signer (
    signer_id integer
        PRIMARY KEY
        GENERATED ALWAYS AS IDENTITY,

    instruction_id integer NOT NULL
        REFERENCES cothority.instruction(instruction_id)
        ON UPDATE CASCADE ON DELETE CASCADE,

    signature bytea NOT NULL,
    counter integer NOT NULL,
    identity character varying NOT NULL
);


ALTER TABLE cothority.signer OWNER TO bypros;

--
-- Read-only user
--

CREATE USER proxy WITH PASSWORD '1234';
GRANT CONNECT ON DATABASE bypros TO proxy;
GRANT USAGE ON SCHEMA cothority TO proxy;
GRANT SELECT ON ALL TABLES IN SCHEMA cothority TO proxy;
