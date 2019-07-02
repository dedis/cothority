import { Message, util } from "protobufjs/light";
import shuffle from "shuffle-array";
import Log from "../log";
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

    /**
     * Set the timeout value for new connections
     * @param value Timeout in milliseconds
     */
    setTimeout(value: number): void;
}

/**
 * Single peer connection
 */
export class WebSocketConnection implements IConnection {
    private url: string;
    private service: string;
    private timeout: number;

    /**
     * @param addr      Address of the distant peer
     * @param service   Name of the service to reach
     */
    constructor(addr: string, service: string) {
        this.url = addr;
        this.service = service;
        this.timeout = 30 * 1000; // 30s by default
    }

    /** @inheritdoc */
    getURL(): string {
        return this.url;
    }

    /** @inheritdoc */
    setTimeout(value: number): void {
        this.timeout = value;
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
            Log.lvl4(`Socket: new WebSocket(${path})`);
            const ws = factory(path);
            const bytes = Buffer.from(message.$type.encode(message).finish());

            const timer = setTimeout(() => ws.close(1002, "timeout"), this.timeout);

            ws.onOpen(() => ws.send(bytes));

            ws.onMessage((data: Buffer) => {
                clearTimeout(timer);
                const buf = Buffer.from(data);
                Log.lvl4("Getting message with length:", buf.length);

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
                    Log.error("Got close:", code, reason);
                    reject(new Error(reason));
                }
            });

            ws.onError((err: Error) => {
                clearTimeout(timer);

                reject(new Error("error in websocket " + path + ": " + err));
            });
        });
    }
}

/**
 * Multi peer connection that tries all nodes one after another. It can send the command to more
 * than one node in parallel and return the first success if 'parallel' i > 1.
 */
export class RosterWSConnection {
    private addresses: string[];
    private connectionsActive: WebSocketConnection[];
    private connectionsPool: WebSocketConnection[];

    /**
     * @param r         The roster to use
     * @param service   The name of the service to reach
     * @param parallel how many nodes to contact in parallel
     */
    constructor(r: Roster, private service: string, parallel: number = 2) {
        if (parallel < 1) {
            throw new Error("parallel must be >= 1");
        }
        this.addresses = r.list.map((conode) => conode.getWebSocketAddress());
        shuffle(this.addresses);
        // Initialize the pool of connections
        this.connectionsPool = this.addresses.map((addr) => new WebSocketConnection(addr, service));
        // And take the first 'parallel' connections
        this.connectionsActive = this.connectionsPool.splice(0, parallel);
        // Upon failure of a connection, it is pushed to the end of the connectionsPool, and a
        // new connection is taken from the beginning of the connectionsPool.
    }

    /**
     * Sends a message to conodes in parallel. As soon as one of the conodes returns
     * success, the message is returned. If a conode returns an error (or times out),
     * a next conode from this.addresses is contacted. If all conodes return an error,
     * the promise is rejected.
     *
     * @param message the message to send
     * @param reply the type of the message to return
     */
    async send<T extends Message>(message: Message, reply: typeof Message): Promise<T> {
        const errors: string[] = [];
        let rotate = this.addresses.length - this.connectionsActive.length;

        // Get the first reply - need to take care not to return a reject too soon, else
        // all other promises will be ignored.
        // The promises that never 'resolve' or 'reject' will later be collected by GC:
        // https://stackoverflow.com/questions/36734900/what-happens-if-we-dont-resolve-or-reject-the-promise
        return Promise.race(this.connectionsActive.map((_, i) => {
            return new Promise<T>(async (resolve, reject) => {
                do {
                    try {
                        const sub = await this.connectionsActive[i].send(message, reply);
                        // Signal to other connections that have an error that they don't need
                        // to retry.
                        rotate = -1;
                        resolve(sub as T);
                    } catch (e) {
                        errors.push(e);
                        if (errors.length === this.addresses.length) {
                            // It's the last connection that also threw an error, so let's quit
                            reject(errors);
                        }
                        rotate--;
                        if (rotate >= 0) {
                            // Take the oldest connection that hasn't been used yet
                            this.connectionsPool.push(this.connectionsActive[i]);
                            this.connectionsActive[i] = this.connectionsPool.shift();
                        }
                    }
                } while (rotate >= 0);
            });
        }));
    }

    /**
     * To be conform with an IConnection
     */
    getURL(): string {
        return this.connectionsActive[0].getURL();
    }

    /**
     * To be conform with an IConnection - sets the timeout on all connections.
     */
    setTimeout(value: number) {
        this.connectionsPool.forEach((conn) => {
            conn.setTimeout(value);
        });
        this.connectionsActive.forEach((conn) => {
            conn.setTimeout(value);
        });
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
