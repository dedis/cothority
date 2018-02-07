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
     * @param publicKey public key in bytes format
     * @param address tcp address of the cothority node
     * @return a ServerIdentity
     * */
    constructor(publicKey, address) {
        this.pub = publicKey
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
     * Returns a new ServerIdentity from the public key in hexadecimal format
     * and address
     * @param {string} hex-encoded public key
     * @param {string} address
     * @return a ServerIdentity
     * */
    static fromHexPublic(hexPublic,address) {
        const pub = misc.hexToUint8Array(hexPublic);
        return new ServerIdentity(pub,address);
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
    * // where public has to be a hex-encoded string.
    *
    *    [[servers]]
    *        Address = "tcp://127.0.0.1:7001"
    *        Public = "4e3008c1a2b6e022fb60b76b834f174911653e9c9b4156cc8845bfb334075655"
    *        Description = "conode1"
    *    [[servers]]
    *        Address = "tcp://127.0.0.1:7003"
    *        Public = "e5e23e58539a09d3211d8fa0fb3475d48655e0c06d83e93c8e6e7d16aa87c106"
    *        Description = "conode2"
    *
    * @param {string} toml of the above format.
    *
    * @throws {TypeError} when toml is not a string
    * @return {Roster} roster
    */
    static fromTOML(toml) {
        if (typeof toml !== 'string')
            throw new TypeError;

        const roster = topl.parse(toml);
        const identities = roster.servers.map((server) => ServerIdentity.fromHexPublic(server.Public,server.Address));
        return new Roster(identities);
    }
}

module.exports = {
    Roster,
    ServerIdentity,
};
