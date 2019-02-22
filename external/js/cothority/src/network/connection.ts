import { Message, util } from "protobufjs/light";
import shuffle from "shuffle-array";
import Logger from "../log";
import { Roster } from "./proto";
import { BrowserWebSocketAdapter, WebSocketAdapter } from "./websocket-adapter";

let factory: (path: string) => WebSocketAdapter = (path: string) => new BrowserWebSocketAdapter(path);

/**
 * Set the websocket generator. The default one is compatible
 * with browsers and nodejs.
 * @param generator A function taking a path and creating a websocket adapter instance
 */
export function setFactory(generator: (path: string) => WebSocketAdapter): void {
    factory = generator;
}

/**
 * A connection allows to send a message to one or more distant peer
 */
export interface IConnection {
    /**
     * Send a message to the distant peer
     * @param message   Protobuf compatible message
     * @param reply     Protobuf type of the reply
     * @returns a promise resolving with the reply on success, rejecting otherwise
     */
    send<T extends Message>(message: Message, reply: typeof Message): Promise<T>;

    /**
     * Get the complete distant address
     * @returns the address as a string
     */
    getURL(): string;
}

/**
 * Single peer connection
 */
export class WebSocketConnection implements IConnection {
    protected url: string;
    private service: string;

    /**
     * @param addr      Address of the distant peer
     * @param service   Name of the service to reach
     */
    constructor(addr: string, service: string) {
        this.url = addr;
        this.service = service;
    }

    /** @inheritdoc */
    getURL(): string {
        return this.url;
    }

    /** @inheritdoc */
    async send<T extends Message>(message: Message, reply: typeof Message): Promise<T> {
        if (!message.$type) {
            return Promise.reject(new Error(`message "${message.constructor.name}" is not registered`));
        }

        if (!reply.$type) {
            return Promise.reject(new Error(`message "${reply}" is not registered`));
        }

        return new Promise((resolve, reject) => {
            const path = this.url + "/" + this.service + "/" + message.$type.name.replace(/.*\./, "");
            Logger.lvl4(`Socket: new WebSocket(${path})`);
            const ws = factory(path);
            const bytes = Buffer.from(message.$type.encode(message).finish());

            ws.onOpen(() => {
                ws.send(bytes);
            });

            ws.onMessage((data: Buffer) => {
                const buf = Buffer.from(data);
                Logger.lvl4("Getting message with length:", buf.length);

                try {
                    const ret = reply.decode(buf) as T;

                    resolve(ret);
                } catch (err) {
                    if (err instanceof util.ProtocolError) {
                        reject(err);
                    } else {
                        reject(
                            new Error(`Error when trying to decode the message "${reply.$type.name}": ${err.message}`),
                        );
                    }
                }

                ws.close(1000);
            });

            ws.onClose((code: number, reason: string) => {
                if (code !== 1000) {
                    Logger.error("Got close:", code, reason);
                    reject(new Error(reason));
                }
            });

            ws.onError((err: Error) => {
                reject(new Error("error in websocket " + path + ": " + err));
            });
        });
    }
}

/**
 * Multi peer connection that tries all nodes one after another
 */
export class RosterWSConnection extends WebSocketConnection {
    addresses: string[];

    /**
     * @param r         The roster to use
     * @param service   The name of the service to reach
     */
    constructor(r: Roster, service: string) {
        super("", service);
        this.addresses = r.list.map((conode) => conode.getWebSocketAddress());
    }

    /** @inheritdoc */
    async send<T extends Message>(message: Message, reply: typeof Message): Promise<T> {
        const addresses = this.addresses.slice();
        shuffle(addresses);

        for (const addr of addresses) {
            this.url = addr;

            try {
                // we need to await here to catch and try another conode
                return await super.send(message, reply);
            } catch (e) {
                Logger.lvl3(`fail to send on ${addr} with error:`, e);
            }
        }

        throw new Error("no conodes are available or all conodes returned an error");
    }
}

/**
 * Single peer connection that reaches only the leader of the roster
 */
export class LeaderConnection extends WebSocketConnection {
    /**
     * @param roster    The roster to use
     * @param service   The name of the service
     */
    constructor(roster: Roster, service: string) {
        if (roster.list.length === 0) {
            throw new Error("Roster should have at least one node");
        }

        super(roster.list[0].address, service);
    }
}
