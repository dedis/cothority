import { Message, util } from "protobufjs/light";
import URL from "url-parse";
import Log from "../log";
import { Nodes } from "./nodes";
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
 * A connection allows to send a message to one or more distant peers. It has a default
 * service it is connected to, and can only be used to send messages to that service.
 * To change service, a copy has to be created using the copy method.
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

    /**
     * Sets how many nodes will be contacted in parallel
     * @param p number of nodes to contact in parallel
     */
    setParallel(p: number): void;

    /**
     * Creates a copy of the connection, but usable with the given service.
     * @param service
     */
    copy(service: string): IConnection;
}

/**
 * Single peer connection to one single node.
 */
export class WebSocketConnection implements IConnection {
    private readonly url: string;
    private readonly service: string;
    private timeout: number;

    /**
     * @param addr      Address of the distant peer
     * @param service   Name of the service to reach
     */
    constructor(addr: string, service: string) {
        const url = new URL(addr, {});
        if (typeof globalThis !== "undefined" && typeof globalThis.location !== "undefined") {
            if (globalThis.location.protocol === "https:") {
                url.set("protocol", "wss");
            }
        }
        this.url = url.href;

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

            const timer = setTimeout(() => ws.close(1000, "timeout"), this.timeout);

            ws.onOpen(() => {
                Log.lvl3("Sending message to", path);
                ws.send(bytes);
            });

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
                // nativescript-websocket on iOS doesn't return error-code 1002 in case of error, but sets the 'reason'
                // to non-null in case of error.
                if (code !== 1000 || reason) {
                    reject(new Error(reason));
                }
            });

            ws.onError((err: Error) => {
                clearTimeout(timer);

                reject(new Error("error in websocket " + path + ": " + err));
            });
        });
    }

    setParallel(p: number): void {
        if (p > 1) {
            throw new Error("Single connection doesn't support more than one parallel");
        }
    }

    copy(service: string): IConnection {
        return new WebSocketConnection(this.url, service);
    }
}

/**
 * Multi peer connection that tries all nodes one after another. It can send the command to more
 * than one node in parallel and return the first success if 'parallel' i > 1.
 *
 * It uses the Nodes class to manage which nodes will be contacted.
 */
export class RosterWSConnection implements IConnection {
    // Can be set to override the default parallel value
    static defaultParallel: number = 3;
    // debugging variable
    private static totalConnNbr = 0;
    private static nodes: Map<string, Nodes> = new Map<string, Nodes>();
    nodes: Nodes;
    private readonly connNbr: number;
    private msgNbr = 0;
    private parallel: number;
    private rID: string;

    /**
     * @param r         The roster to use or the rID of the Nodes
     * @param service   The name of the service to reach
     * @param parallel  How many nodes to contact in parallel. Can be changed afterwards
     */
    constructor(r: Roster | string, private service: string, parallel: number = RosterWSConnection.defaultParallel) {
        this.setParallel(parallel);
        if (r instanceof Roster) {
            this.rID = r.id.toString("hex");
            if (!RosterWSConnection.nodes.has(this.rID)) {
                RosterWSConnection.nodes.set(this.rID, new Nodes(r));
            }
        } else {
            this.rID = r;
            if (!RosterWSConnection.nodes.has(this.rID)) {
                throw new Error("unknown roster-ID");
            }
        }
        this.nodes = RosterWSConnection.nodes.get(this.rID);
        this.connNbr = RosterWSConnection.totalConnNbr;
        RosterWSConnection.totalConnNbr++;
    }

    /**
     * Set a new parameter for how many nodes should be contacted in parallel.
     * @param p
     */
    setParallel(p: number) {
        if (p < 1) {
            throw new Error("Parallel needs to be bigger or equal to 1");
        }
        this.parallel = p;
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
        const msgNbr = this.msgNbr;
        this.msgNbr++;
        const list = this.nodes.newList(this.service, this.parallel);
        const pool = list.active;

        Log.lvl3(`${this.connNbr}/${msgNbr}`, "sending", message.constructor.name, "with list:",
            pool.map((conn) => conn.getURL()));

        // Get the first reply - need to take care not to return a reject too soon, else
        // all other promises will be ignored.
        // The promises that never 'resolve' or 'reject' will later be collected by GC:
        // https://stackoverflow.com/questions/36734900/what-happens-if-we-dont-resolve-or-reject-the-promise
        return Promise.race(pool.map((conn) => {
            return new Promise<T>(async (resolve, reject) => {
                do {
                    const idStr = `${this.connNbr}/${msgNbr.toString()}: ${conn.getURL()}`;
                    try {
                        Log.lvl3(idStr, "sending");
                        const sub = await conn.send(message, reply);
                        Log.lvl3(idStr, "received OK");

                        if (list.done(conn) === 0) {
                            Log.lvl3(idStr, "first to receive");
                            resolve(sub as T);
                        }
                        return;
                    } catch (e) {
                        Log.lvl3(idStr, "has error", e);
                        errors.push(e);
                        conn = list.replace(conn);
                        if (errors.length >= list.length / 2) {
                            // More than half of the nodes threw an error - quit.
                            reject(errors);
                        }
                    }
                } while (conn !== undefined);
            });
        }));
    }

    /**
     * To be conform with an IConnection
     */
    getURL(): string {
        return this.nodes.newList(this.service, 1).active[0].getURL();
    }

    /**
     * To be conform with an IConnection - sets the timeout on all connections.
     */
    setTimeout(value: number) {
        this.nodes.setTimeout(value);
    }

    copy(service: string): IConnection {
        return new RosterWSConnection(this.rID, service, this.parallel);
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

        super(roster.list[0].getWebSocketAddress(), service);
    }
}
