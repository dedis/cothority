import { Message } from "protobufjs/light";
import { Observable } from "rxjs";
import Log from "../log";
import { IConnection, Nodes } from "./nodes";
import { Roster } from "./proto";
import { WebSocketConnection } from "./websocket";
import { WebSocketAdapter } from "./websocket-adapter";

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

    /** @inheritdoc */
    sendStream<T extends Message>(message: Message, reply: typeof Message):
        Observable<[T, WebSocketAdapter]> {
        const list = this.nodes.newList(this.service, this.parallel);
        return list.active[0].sendStream(message, reply);
    }

    /**
     * Return a new RosterWSConnection for the given service.
     * @param service
     */
    copy(service: string): RosterWSConnection {
        return new RosterWSConnection(this.rID, service, this.parallel);
    }

    /**
     * Invalidate a given address.
     * @param address
     */
    invalidate(address: string): void {
        this.nodes.gotError(address);
    }

    /**
     * Update roster for this connection.
     * @param r
     */
    setRoster(r: Roster) {
        const newID = r.id.toString("hex");
        if (newID !== this.rID) {
            this.rID = newID;
            if (!RosterWSConnection.nodes.has(this.rID)) {
                RosterWSConnection.nodes.set(this.rID, new Nodes(r, this.nodes));
            }
            this.nodes = RosterWSConnection.nodes.get(this.rID);
        }
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
