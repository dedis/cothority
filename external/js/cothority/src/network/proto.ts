import { Point, PointFactory } from "@dedis/kyber";
import { createHash } from "crypto";
import { Message, Properties } from "protobufjs/light";
import UUID from "pure-uuid";
import toml from "toml";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";

const BASE_URL_WS = "ws://";
const BASE_URL_TLS = "tls://";
const URL_PORT_SPLITTER = ":";
const PORT_MIN = 0;
const PORT_MAX = 65535;

/**
 * List of server identities
 */
export class Roster extends Message<Roster> {

    /**
     * Get the length of the roster
     * @returns the length as a number
     */
    get length(): number {
        return this.list.length;
    }

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Roster", Roster, ServerIdentity);
    }

    static fromBytes(b: Buffer): Roster {
        return Roster.decode(b);
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
     * @param data toml with the above format
     * @returns the parsed roster
     */
    static fromTOML(data: string): Roster {
        const roster = toml.parse(data);
        const list = roster.servers.map((server: any) => {
            const {Public, Suite, Address, Description, Services, Url} = server;
            const p = PointFactory.fromToml(Suite, Public);

            return new ServerIdentity({
                address: Address,
                description: Description,
                public: p.toProto(),
                serviceIdentities: Object.keys(Services || {}).map((key) => {
                    const {Public: pub, Suite: suite} = Services[key];
                    const point = PointFactory.fromToml(suite, pub);

                    return new ServiceIdentity({name: key, public: point.toProto(), suite});
                }),
                url: Url,
            });
        });

        return new Roster({list});
    }
    readonly id: Buffer;
    readonly list: ServerIdentity[];
    readonly aggregate: Buffer;
    private _agg: Point;

    constructor(properties?: Properties<Roster>) {
        super(properties);

        if (!properties) {
            return;
        }

        const {id, list, aggregate} = properties;

        if (!id || !aggregate) {
            const h = createHash("sha256");
            list.forEach((srvid) => {
                h.update(srvid.getPublic().marshalBinary());

                for (const srviceId of srvid.serviceIdentities) {
                    h.update(srviceId.getPublic().marshalBinary());
                }

                if (!this._agg) {
                    this._agg = srvid.getPublic();
                } else {
                    this._agg.add(this._agg, srvid.getPublic());
                }
            });

            // protobuf fields need to be initialized if we want to encode later
            this.aggregate = this._agg.toProto();
            this.id = Buffer.from(new UUID(5, "ns:URL", h.digest().toString("hex")).export());
        }
    }

    /**
     * Get the public keys for a given service
     *
     * @param name The name of the service
     * @returns the list of points
     */
    getServicePublics(name: string): Point[] {
        return this.list.map((srvid) => {
            const t = srvid.serviceIdentities.find((s) => s.name === name);

            return PointFactory.fromProto(t.public);
        });
    }

    /**
     * Returns the list of public keys of the conodes in the roster.
     */
    getPublics(): Point[] {
        return this.list.map((si) => si.getPublic());
    }

    /**
     * Get a subset of the roster
     *
     * @param start Index of the first identity
     * @param end   Index of the last identity, not inclusive
     * @returns the new roster
     */
    slice(start: number, end?: number): Roster {
        return new Roster({list: this.list.slice(start, end)});
    }

    /**
     * Helper to encode the Roster using protobuf
     * @returns the bytes
     */
    toBytes(): Buffer {
        return Buffer.from(Roster.encode(this).finish());
    }
}

/**
 * Identity of a conode
 */
export class ServerIdentity extends Message<ServerIdentity> {

    /**
     * Converts an HTTP-S URL to a Wesocket URL. It converts 'http' to 'ws' and 'https' to 'wss'.
     * Any other protocols are forbidden and will raise an error. It also removes any trailing '/'.
     * Here are some examples:
     *      http://example.com:77        => ws://example.com:77
     *      https://example.com/path/    => wss:example.com/path
     *      https://example.com:443/     => wss:example.com
     *      tcp://127.0.0.1              => Error
     * Note: It will NOT include the given port in the case it's the default one (for example 80 or 443).
     * Note: In the case there are many slashes at the end of the url, it will only remove one.
     * @param url   the given url field
     * @returns a websocket url
     */
    static urlToWebsocket(url: string): string {
        if (url.match(/^https?:\/\//) == null) {
            throw new Error("The url field should use either 'http:' or 'https:'");
        }
        return url.replace(/^http(.*?)\/?$/, "ws$1");
    }

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("ServerIdentity", ServerIdentity, ServiceIdentity);
    }

    /**
     * Checks wether the address given as parameter has the right format.
     * @param address the address to check
     * @returns true if and only if the address has the right format
     */
    static isValidAddress(address: string): boolean {
        if (address.startsWith(BASE_URL_TLS)) {
            const [, ...array] = address.replace(BASE_URL_TLS, "").split(URL_PORT_SPLITTER);

            if (array.length === 1) {
                const port = parseInt(array[0], 10);

                // Port equal to PORT_MAX is not allowed since the port will be
                // increased by one for the websocket urlRegistered.
                return PORT_MIN <= port && port < PORT_MAX;
            }
        }
        return false;
    }

    /**
     * Converts a TLS URL to a Wesocket URL and builds a complete URL with the path given as parameter.
     * @param address   the server identity to take the urlRegistered from
     * @param path      the path after the base urlRegistered
     * @returns a websocket address
     */
    static addressToWebsocket(address: string, path: string = ""): string {
        const [ip, portStr] = address.replace(BASE_URL_TLS, "").split(URL_PORT_SPLITTER);
        const port = parseInt(portStr, 10) + 1;

        return BASE_URL_WS + ip + URL_PORT_SPLITTER + port + path;
    }
    readonly public: Buffer;
    readonly id: Buffer;
    readonly address: string;
    readonly description: string;
    readonly serviceIdentities: ServiceIdentity[];
    readonly url: string;
    private _point: Point;

    constructor(properties?: Properties<ServerIdentity>) {
        super(properties);

        if (!properties) {
            return;
        }

        if (!properties.id) {
            const hex = this.getPublic().toString();
            this.id = Buffer.from(new UUID(5, "ns:URL", `https://dedis.epfl.ch/id/${hex}`).export());
        }
    }

    /**
     * Get the public key of the server as a Point
     * @returns the point
     */
    getPublic(): Point {
        if (!this._point) {
            // cache the point to avoid multiple unmarshaling
            this._point = PointFactory.fromProto(this.public);
        }

        return this._point.clone();
    }

    /**
     * Returns websocket version of this.url if set, otherwise converts the server
     * address to match the websocket format.
     * @returns the websocket address
     */
    getWebSocketAddress(): string {
        if (this.url) {
            return ServerIdentity.urlToWebsocket(this.url);
        } else {
            return ServerIdentity.addressToWebsocket(this.address);
        }
    }
}

/**
 * Identity of a service for a specific conode. Some services have their own
 * key pair and don't the default one.
 */
export class ServiceIdentity extends Message<ServiceIdentity> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("ServiceIdentity", ServiceIdentity);
    }
    readonly name: string;
    readonly suite: string;
    readonly public: Buffer;
    private _point: Point;

    constructor(properties: Properties<ServiceIdentity>) {
        super(properties);

        this.public = Buffer.from(this.public || EMPTY_BUFFER);
    }

    /**
     * Get the public key as a Kyber point
     *
     * @returns the public key
     */
    getPublic(): Point {
        if (!this._point) {
            this._point = PointFactory.fromProto(this.public);
        }

        return this._point;
    }
}

Roster.register();
ServerIdentity.register();
ServiceIdentity.register();
