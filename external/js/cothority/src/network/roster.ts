const toml = require("toml");
const UUID = require("pure-uuid");
import { createHash } from 'crypto';
import { Point, curve } from '@dedis/kyber';
import { Message, Properties } from 'protobufjs';
import { registerMessage } from '../protobuf';

const ed25519 = new curve.edwards25519.Curve();

export class Roster extends Message<Roster> {
    id: Buffer;
    list: ServerIdentity[];
    aggregate: string;

    constructor(properties?: Properties<Roster>) {
        super(properties);

        const { id, list, aggregate } = properties;

        this.id = id;
        this.list = list;
        this.aggregate = aggregate;
    }

    /*
    constructor(list: ServerIdentity[]) {
        super();
        this.list = list;
        const h = createHash("sha256");
        list.forEach((l) => {
            h.update(l.public.marshalBinary());

            if (!this.aggregate) {
                this.aggregate = l.public;
            } else {
                this.aggregate.add(this.aggregate, l.public);
            }
        });
        this.id = new UUID(5, "ns:URL", h.digest().toString('hex')).export();
    }
    */

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
     * @param {kyber.Group} group to construct the identities
     * @param {string} toml of the above format.
     * @param {boolean} wss to connect using WebSocket Secure (port 443)
     *
     * @throws {TypeError} when toml is not a string
     * @return {Roster} roster
     */
    static fromTOML(data: string | Buffer, wss: boolean = false): any {
        const roster = toml.parse(data);
        const list = roster.servers.map((server: any) => {
            const { Public, Address, Description } = server;
            return new ServerIdentity({ public: Public, address: Address, description: Description });
        });

        return new Roster({ list });
    }
}

export class ServerIdentity extends Message<ServerIdentity> {
    readonly public: string;
    readonly id: Buffer;
    readonly address: string;
    readonly description: string;

    constructor(properties?: Properties<ServerIdentity>) {
        super(properties);

        if (!properties) {
            return;
        }

        if (!properties.id) {
            this.id = new UUID(5, 'ns:URL', `https://dedis.epfl.ch/id/${this.public}`).export();
        }
    }

    getPoint(): Point {
        const buf = Buffer.from(this.public);
        const pub = ed25519.point();
        pub.unmarshalBinary(buf);
        return pub;
    }

    toWebsocket(path: string): string {
        return ServerIdentity.addressToWebsocket(this.address, path);
        //.replace("pop.dedis.ch", "192.168.0.1");
    }

    /**
     * Checks wether the address given as parameter has the right format.
     * @param {string} address - the address to check
     * @returns {boolean} - true if and only if the address has the right format
     */
    static isValidAddress(address: string): boolean {
        const BASE_URL_TLS = "tls://";
        const URL_PORT_SPLITTER = ":";
        const PORT_MIN = 0;
        const PORT_MAX = 65535;

        if (address.startsWith(BASE_URL_TLS)) {
            let [ip, ...array] = address.replace(BASE_URL_TLS, "").split(URL_PORT_SPLITTER);

            if (array.length === 1) {
                const port = parseInt(array[0]);

                // Port equal to PORT_MAX is not allowed since the port will be increased by one for the websocket urlRegistered.
                return PORT_MIN <= port && port < PORT_MAX;
            }
        }
        return false;
    }

    /**
     * Converts a TLS URL to a Wesocket URL and builds a complete URL with the path given as parameter.
     * @param {ServerIdentity|string} serverIdentity - the server identity to take the urlRegistered from
     * @param {string} path - the path after the base urlRegistered
     * @returns {string} - the builded websocket urlRegistered
     */
    static addressToWebsocket(address: string, path: string): string {
        const URL_PORT_SPLITTER = ":";
        const BASE_URL_WS = "ws://";
        const BASE_URL_TLS = "tls://";

        let [ip, portStr] = address.replace(BASE_URL_TLS, "").split(URL_PORT_SPLITTER);
        let port = parseInt(portStr) + 1;

        return BASE_URL_WS + ip + URL_PORT_SPLITTER + port + path;
    }
}

registerMessage('ServerIdentity', ServerIdentity);
