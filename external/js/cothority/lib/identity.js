"use strict";

const topl = require('topl');
const UUID = require('pure-uuid');
const protobuf = require('protobufjs')

const misc = require('./misc');


/**
 * ServerIdentity represents a cothority server. It has an associated public key
 * and a TCP address. The WebSocket address is derived from the TCP address.
 * */
class ServerIdentity {
    /*
     * Returns a new ServerIdentity from the public key and address.
     * @param hexPublic public key in hexadecimal format
     * @param address tcp address of the cothority node
     * @return a ServerIdentity 
     * */
    constructor(hexPublic, address) {
        this.pub = misc.hexToUint8Array(hexPublic);
        this.addr = address;
        // id of the identity
        const url = 'https://dedis.epfl.ch/id/' + misc.uint8ArrayToHex(this.pub);
        this._id = new UUID(5, 'ns:URL', url).export();
        // tcp + websocket address
        let parts = address.replace("tcp://", "").split(":");
        parts[1] = parseInt(parts[1]) + 1;
        this.wsAddr = "ws://" + parts.join(":");
    }

    /*
     * @return the public key as a Uint8Array buffer
     * */
    get public() {
        return this.pub;
    }

    /*
     * @return the WebSocket address. That can be passed into the net.Socket
     * address constructor argument.
     * */
    get websocketAddr() {
        return this.wsAddr; 
    }

    /*
     * @return the tcp address of the server
     * */
    get tcpAddr() {
        return this.addr;
    }

    /*
     * @return the id of this serveridentity
     * */
    get id() {
        return this._id;
    }

    toString() {
        return this.tcpAddr;
    }
}


/*
 * Roster represents a group of servers. It basically consists in a list of
 * ServerIdentity with some helper functions.
 * */
class Roster {

    /*
     * @param a list of ServerIdentity
     * @return a Roster from the given list of identites
     * */
    constructor(identities) {
        this._identities = identities
    }

    /*
     * Random selects a random identity and returns it
     * @return a random identity
     * */
    random() {
        const idx = Math.floor(Math.random() * (this.length()-1));
        return this._identities[idx];
    }
   
    /*
     * @return the list of identities composing this Roster
     * */
    get identities() {
        return this._identities;
    }

    /*
     * @return the length of the roster
     * */
    get length() {
        return this._identities.length;
    }

    /*
     * @return the id of the roster
     * */
    get id() {
        throw new Error("not implemented yet");
    }

    /**
    * Parse cothority roster toml string into a Roster object.
    * @example
    * // Toml needs to adhere to the following format
    * // where public has to be a base64 encodable string.
    *
    *     [[servers]]
    *         Address = "tcp://127.0.0.1:7002"
    *         Public = "GhxOf6H+23gK2NP4qu+FrRT5/Ca08+tCRcAaoZu26BY="
    *         Description = "Conode_1"
    *     [[servers]]
    *         Address = "tcp://127.0.0.1:7004"
    *         Public = "HSSppBPaE4QFpPQ2yvDN9Fss/RIe/jmtEvNvMm3y49M="
    *         Description = "Conode_2"
    *
    * Where public has to be a base64 encodable string.
    * @param {string} toml of the above format.
    *
    * @throws {TypeError} when toml is not a string
    * @return {Roster} roster
    */
    static fromTOML(toml) {
        if (typeof toml !== 'string')
            throw new TypeError;

        const roster = topl.parse(toml);
        const identities = roster.servers.map((server) => new ServerIdentity(server.Public,server.Address));
        return new Roster(identities);
    }
}

module.exports = {
    Roster,
    ServerIdentity,
};
