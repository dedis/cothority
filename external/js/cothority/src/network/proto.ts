const toml = require("toml");
const UUID = require("pure-uuid");
import { createHash } from 'crypto';
import { Point, PointFactory } from '@dedis/kyber';
import { Message, Properties } from 'protobufjs';
import { registerMessage } from '../protobuf';

export class Roster extends Message<Roster> {
    readonly id: Buffer;
    readonly list: ServerIdentity[];
    readonly aggregate: Buffer;

    private _agg: Point;

    constructor(properties?: Properties<Roster>) {
        super(properties);

        if (!properties) {
            return;
        }

        const { id, list, aggregate } = properties;

        if (!id || !aggregate) {
            const h = createHash("sha256");
            list.forEach((srvid) => {
                h.update(srvid.getPublic().toProto());

                if (!this._agg) {
                    this._agg = srvid.getPublic();
                } else {
                    this._agg.add(this._agg, srvid.getPublic());
                }
            });

            // protobuf fields need to be initialized if we want to encode later
            this.aggregate = this._agg.toProto();
            this.id = new UUID(5, "ns:URL", h.digest().toString('hex')).export();
        }
    }

    get length(): number {
        return this.list.length;
    }

    slice(start: number, end?: number): Roster {
        return new Roster({ list: this.list.slice(start, end) });
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
            const { Public, Suite, Address, Description, Services } = server;
            const p = PointFactory.fromToml(Suite, Public);

            return new ServerIdentity({
                public: p.toProto(),
                address: Address,
                description: Description,
                serviceIdentities: Object.keys(Services).map((key) => {
                    const { Public, Suite: suite } = Services[key];
                    const point = PointFactory.fromToml(suite, Public);

                    return new ServiceIdentity({ name: key, public: point.toProto(), suite });
                }),
            });
        });

        return new Roster({ list });
    }
}

export class ServerIdentity extends Message<ServerIdentity> {
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
            this.id = new UUID(5, 'ns:URL', `https://dedis.epfl.ch/id/${hex}`).export();
        }
    }

    getPublic(): Point {
        if (this._point) {
            // cache the point to avoid multiple unmarshaling
            return this._point;
        }

        const pub = PointFactory.fromProto(this.public);
        this._point = pub;
        return pub;
    }

    getWebSocketAddress(): string {
        return ServerIdentity.addressToWebsocket(this.address, '');
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

export class ServiceIdentity extends Message<ServiceIdentity> {
    readonly name: string;
    readonly suite: string;
    readonly public: Buffer;

    constructor(properties: Properties<ServiceIdentity>) {
        super(properties);
    }
}

registerMessage('Roster', Roster);
registerMessage('ServerIdentity', ServerIdentity);
registerMessage('ServiceIdentity', ServiceIdentity);
