--
-- PostgreSQL database dump
--

-- Dumped from database version 13.2 (Debian 13.2-1.pgdg100+1)
-- Dumped by pg_dump version 13.2

-- Started on 2021-03-16 12:57:24 CET

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

--
-- TOC entry 4 (class 2615 OID 16387)
-- Name: cothority; Type: SCHEMA; Schema: -; Owner: bypros
--

CREATE SCHEMA cothority;

-- ADDED BY NCKR
CREATE USER proxy WITH PASSWORD '1234';
GRANT CONNECT ON DATABASE bypros TO proxy;
GRANT USAGE ON SCHEMA cothority TO proxy;
GRANT SELECT ON ALL TABLES IN SCHEMA cothority TO proxy;
-- END ADDED BY NKCR

ALTER SCHEMA cothority OWNER TO bypros;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 211 (class 1259 OID 16646)
-- Name: argument; Type: TABLE; Schema: cothority; Owner: bypros
--

CREATE TABLE cothority.argument (
    argument_id integer NOT NULL,
    name character varying NOT NULL,
    value bytea,
    instruction_id integer NOT NULL
);


ALTER TABLE cothority.argument OWNER TO bypros;

--
-- TOC entry 212 (class 1259 OID 16659)
-- Name: argument_argument_id_seq; Type: SEQUENCE; Schema: cothority; Owner: bypros
--

ALTER TABLE cothority.argument ALTER COLUMN argument_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME cothority.argument_argument_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 202 (class 1259 OID 16396)
-- Name: block; Type: TABLE; Schema: cothority; Owner: bypros
--

CREATE TABLE cothority.block (
    block_id integer NOT NULL,
    hash bytea NOT NULL
);


ALTER TABLE cothority.block OWNER TO bypros;

--
-- TOC entry 205 (class 1259 OID 16435)
-- Name: block_block_id_seq; Type: SEQUENCE; Schema: cothority; Owner: bypros
--

ALTER TABLE cothority.block ALTER COLUMN block_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME cothority.block_block_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 204 (class 1259 OID 16409)
-- Name: instruction; Type: TABLE; Schema: cothority; Owner: bypros
--

CREATE TABLE cothority.instruction (
    instruction_id integer NOT NULL,
    transaction_id integer NOT NULL,
    type_id integer NOT NULL,
    action character varying NOT NULL,
    instance_iid bytea NOT NULL,
    contract_iid bytea NOT NULL,
    contract_name character varying NOT NULL
);


ALTER TABLE cothority.instruction OWNER TO bypros;

--
-- TOC entry 201 (class 1259 OID 16388)
-- Name: instructionType; Type: TABLE; Schema: cothority; Owner: bypros
--

CREATE TABLE cothority."instructionType" (
    type_id integer NOT NULL,
    name character varying(10) NOT NULL
);

-- ADDED BY NKCR
COPY cothority."instructionType" (type_id, name) FROM stdin;
1	Invalid
2	Spawn
3	Invoke
4	Delete
\.
-- END ADDED BY NKCR

ALTER TABLE cothority."instructionType" OWNER TO bypros;

--
-- TOC entry 207 (class 1259 OID 16439)
-- Name: instructionType_type_id_seq; Type: SEQUENCE; Schema: cothority; Owner: bypros
--

ALTER TABLE cothority."instructionType" ALTER COLUMN type_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME cothority."instructionType_type_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 208 (class 1259 OID 16441)
-- Name: instruction_instruction_id_seq; Type: SEQUENCE; Schema: cothority; Owner: bypros
--

ALTER TABLE cothority.instruction ALTER COLUMN instruction_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME cothority.instruction_instruction_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 210 (class 1259 OID 16633)
-- Name: signer; Type: TABLE; Schema: cothority; Owner: bypros
--

CREATE TABLE cothority.signer (
    signer_id integer NOT NULL,
    signature bytea NOT NULL,
    counter integer NOT NULL,
    instruction_id integer NOT NULL,
    identity character varying NOT NULL
);


ALTER TABLE cothority.signer OWNER TO bypros;

--
-- TOC entry 209 (class 1259 OID 16631)
-- Name: signer_signer_id_seq; Type: SEQUENCE; Schema: cothority; Owner: bypros
--

ALTER TABLE cothority.signer ALTER COLUMN signer_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME cothority.signer_signer_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 203 (class 1259 OID 16404)
-- Name: transaction; Type: TABLE; Schema: cothority; Owner: bypros
--

CREATE TABLE cothority.transaction (
    transaction_id integer NOT NULL,
    accepted boolean NOT NULL,
    block_id integer NOT NULL
);


ALTER TABLE cothority.transaction OWNER TO bypros;

--
-- TOC entry 206 (class 1259 OID 16437)
-- Name: transaction_transaction_id_seq; Type: SEQUENCE; Schema: cothority; Owner: bypros
--

ALTER TABLE cothority.transaction ALTER COLUMN transaction_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME cothority.transaction_transaction_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 2842 (class 2606 OID 16481)
-- Name: block Block PK; Type: CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.block
    ADD CONSTRAINT "Block PK" PRIMARY KEY (block_id);


--
-- TOC entry 2846 (class 2606 OID 16483)
-- Name: instruction Instruction PK; Type: CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.instruction
    ADD CONSTRAINT "Instruction PK" PRIMARY KEY (instruction_id);


--
-- TOC entry 2840 (class 2606 OID 16395)
-- Name: instructionType InstructionType_pkey; Type: CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority."instructionType"
    ADD CONSTRAINT "InstructionType_pkey" PRIMARY KEY (type_id);


--
-- TOC entry 2844 (class 2606 OID 16479)
-- Name: transaction Transaction PK; Type: CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.transaction
    ADD CONSTRAINT "Transaction PK" PRIMARY KEY (transaction_id);


--
-- TOC entry 2850 (class 2606 OID 16653)
-- Name: argument argument_pkey; Type: CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.argument
    ADD CONSTRAINT argument_pkey PRIMARY KEY (argument_id);


--
-- TOC entry 2848 (class 2606 OID 16637)
-- Name: signer signer_pkey; Type: CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.signer
    ADD CONSTRAINT signer_pkey PRIMARY KEY (signer_id);


--
-- TOC entry 2854 (class 2606 OID 16641)
-- Name: signer Instruction; Type: FK CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.signer
    ADD CONSTRAINT "Instruction" FOREIGN KEY (instruction_id) REFERENCES cothority.instruction(instruction_id) ON UPDATE CASCADE ON DELETE CASCADE NOT VALID;


--
-- TOC entry 2855 (class 2606 OID 16654)
-- Name: argument Instruction; Type: FK CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.argument
    ADD CONSTRAINT "Instruction" FOREIGN KEY (instruction_id) REFERENCES cothority.instruction(instruction_id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 2851 (class 2606 OID 16494)
-- Name: transaction block; Type: FK CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.transaction
    ADD CONSTRAINT block FOREIGN KEY (block_id) REFERENCES cothority.block(block_id) ON UPDATE CASCADE ON DELETE CASCADE NOT VALID;


--
-- TOC entry 2853 (class 2606 OID 16504)
-- Name: instruction transaction; Type: FK CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.instruction
    ADD CONSTRAINT transaction FOREIGN KEY (transaction_id) REFERENCES cothority.transaction(transaction_id) ON UPDATE CASCADE ON DELETE CASCADE NOT VALID;


--
-- TOC entry 2852 (class 2606 OID 16499)
-- Name: instruction type; Type: FK CONSTRAINT; Schema: cothority; Owner: bypros
--

ALTER TABLE ONLY cothority.instruction
    ADD CONSTRAINT type FOREIGN KEY (type_id) REFERENCES cothority."instructionType"(type_id) ON UPDATE CASCADE ON DELETE CASCADE NOT VALID;


--
-- TOC entry 2991 (class 0 OID 0)
-- Dependencies: 4
-- Name: SCHEMA cothority; Type: ACL; Schema: -; Owner: bypros
--

GRANT USAGE ON SCHEMA cothority TO proxy;


--
-- TOC entry 2992 (class 0 OID 0)
-- Dependencies: 211
-- Name: TABLE argument; Type: ACL; Schema: cothority; Owner: bypros
--

GRANT SELECT ON TABLE cothority.argument TO proxy;


--
-- TOC entry 2993 (class 0 OID 0)
-- Dependencies: 202
-- Name: TABLE block; Type: ACL; Schema: cothority; Owner: bypros
--

GRANT SELECT ON TABLE cothority.block TO proxy;


--
-- TOC entry 2994 (class 0 OID 0)
-- Dependencies: 204
-- Name: TABLE instruction; Type: ACL; Schema: cothority; Owner: bypros
--

GRANT SELECT ON TABLE cothority.instruction TO proxy;


--
-- TOC entry 2995 (class 0 OID 0)
-- Dependencies: 201
-- Name: TABLE "instructionType"; Type: ACL; Schema: cothority; Owner: bypros
--

GRANT SELECT ON TABLE cothority."instructionType" TO proxy;


--
-- TOC entry 2996 (class 0 OID 0)
-- Dependencies: 210
-- Name: TABLE signer; Type: ACL; Schema: cothority; Owner: bypros
--

GRANT SELECT ON TABLE cothority.signer TO proxy;


--
-- TOC entry 2997 (class 0 OID 0)
-- Dependencies: 203
-- Name: TABLE transaction; Type: ACL; Schema: cothority; Owner: bypros
--

GRANT SELECT ON TABLE cothority.transaction TO proxy;


-- Completed on 2021-03-16 12:57:25 CET

--
-- PostgreSQL database dump complete
--

