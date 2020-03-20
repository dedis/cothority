import { Message } from "protobufjs";
import { Observable } from "rxjs";
import shuffle from "shuffle-array";
import Log from "../log";
import { Roster } from "./proto";
import { WebSocketConnection } from "./websocket";
import { WebSocketAdapter } from "./websocket-adapter";

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
     * Get the origin of the distant address
     * @returns the address as a string
     */
    getURL(): string;

    /**
     * Set the timeout value for new connections
     * @param value Timeout in milliseconds
     */
    setTimeout(value: number): void;

    /**
     * Creates a copy of the connection, but usable with the given service.
     * @param service
     */
    copy(service: string): IConnection;

    /**
     * Send a message to the distant peer
     * @param message Protobuf compatible message
     * @param reply Protobuf type of the reply
     */
    sendStream<T extends Message>(message: Message, reply: typeof Message):
        Observable<[T, WebSocketAdapter]>;

    /**
     * Sets how many nodes will be contacted in parallel
     * @deprecated - don't use IConnection for that, but rather directly a
     * RosterWSConnection.
     * @param p number of nodes to contact in parallel
     */
    setParallel(p: number): void;
}

/**
 * Nodes holds all nodes for all services in two lists - one active for the number of
 * parallel open connections, and one reserve pool for connections that can take over if the
 * active list fails.
 *
 * It does some advanced checking of which nodes to contact using the following steps:
 * - sorting the nodes to contact by response time - fastest get contacted first
 * - failing nodes are replaced by currently unused nodes
 * - if a node is slower than the slowThreshold, the node is replaced with a node from the reserve pool
 */
export class Nodes {
    // the threshold for the ratio of response_node / response_fastest where the node is
    // put in the 'reserve'.
    static slowThreshold: number = 10;

    // holds all nodes that are available. After each `send`, the nodes will be sorted from fastest to slowest.
    // Each time `gotError` is called, the corresponding node will be moved to the end of the list.
    private readonly nodeList: Node[] = [];

    constructor(r: Roster, previous?: Nodes) {
        const addresses = r.list.map((conode) => conode.getWebSocketAddress());
        if (previous === undefined) {
            shuffle(addresses);
        } else {
            // Keep the order of the addresses, so that the fastest nodes stay at the beginning.
            const fastest: string[] = [];
            for (const addr of previous.nodeList) {
                const i = addresses.findIndex((a) => a === addr.address);
                if (i >= 0) {
                    fastest.push(addresses.splice(i, 1)[0]);
                }
            }
            addresses.unshift(...fastest);
        }
        // Initialize the pool of connections
        this.nodeList = addresses.map((addr) => new Node(addr));
    }

    /**
     * Creates a new NodeList for one message.
     * @param service which service to use
     * @param parallel how many calls will run in parallel
     */
    newList(service: string, parallel: number): NodeList {
        return new NodeList(this, service, parallel);
    }

    /**
     * Marks the node of the given address as having an error. The error must be
     * a websocket-error 1001 or higher. An error in the request itself (refused
     * transaction) should be treated as a passing node.
     * @param address
     */
    gotError(address: string) {
        this.replaceActive(this.index(address));
    }

    /**
     * Marks the node with the given address as having successfully treated the
     * message. It will re-order the nodes to reflect order of arrival. If the
     * node is more than 10x slower than the fastest node, it will be replaced
     * with a node from the reserve queue.
     * @param address node with successful return
     * @param rang order of arrival
     * @param ratio delay in answer between first reply and this reply
     */
    done(address: string, rang: number, ratio: number) {
        const index = this.index(address);
        if (index >= 0) {
            if (ratio >= Nodes.slowThreshold) {
                this.replaceActive(index);
            } else {
                this.swapNodes(index, rang);
            }
        }
    }

    /**
     * Sets the timeout on all nodes.
     * @param t
     */
    setTimeout(t: number) {
        this.nodeList.forEach((n) => n.setTimeout(t));
    }

    /**
     * Returns the IConnections corresponding to the active list and the reserve nodes.
     * @param active how many active nodes to return
     * @param service the chosen service
     */
    splitList(active: number, service: string): [IConnection[], IConnection[]] {
        const wsc = this.nodeList.map((n) => n.getService(service));
        return [wsc.slice(0, active), wsc.slice(active)];
    }

    /**
     * Replaces the given node from the active queue with the first node from
     * the reserve queue.
     * @param index
     */
    private replaceActive(index: number) {
        if (index >= 0) {
            this.nodeList.push(this.nodeList.splice(index, 1)[0]);
        }
    }

    /**
     * Swaps two nodes in the active queue.
     */
    private swapNodes(a: number, b: number) {
        if (a >= 0 && b >= 0 &&
            a < this.nodeList.length && b < this.nodeList.length) {
            [this.nodeList[a], this.nodeList[b]] =
                [this.nodeList[b], this.nodeList[a]];
        } else {
            Log.error("asked to swap", a, b, this.nodeList.length);
        }
    }

    /**
     * Gets the index of a given address.
     * @param address
     */
    private index(address: string): number {
        return this.nodeList.findIndex((c) => {
            return c.address === address;
        });
    }
}

/**
 * A Node holds one IConnection per service.
 */
export class Node {
    private services: Map<string, IConnection> = new Map<string, IConnection>();

    constructor(readonly address: string) {
    }

    /**
     * Returns a IConnection for a given service. If the
     * connection doesn't exist yet, it will be created.
     * @param name
     */
    getService(name: string): IConnection {
        if (this.services.has(name)) {
            return this.services.get(name);
        }
        this.services.set(name, new WebSocketConnection(this.address, name));
        return this.getService(name);
    }

    /**
     * Sets the timeout for all connections of this node.
     */
    setTimeout(t: number) {
        this.services.forEach((conn) => conn.setTimeout(t));
    }
}

/**
 * A NodeList is used to interact with the Nodes-class by allowing
 * the requester to indicate the order of arrival of messages and
 * which nodes didn't reply correctly.
 */
export class NodeList {
    readonly active: IConnection[];
    private reserve: IConnection[];
    private readonly start: number;
    private first: number = 0;
    private replied: number = 0;

    constructor(private nodes: Nodes, service: string, parallel: number) {
        this.start = Date.now();
        [this.active, this.reserve] = nodes.splitList(parallel, service);
    }

    /**
     * Returns the total number of nodes.
     */
    get length(): number {
        return this.active.length + this.reserve.length;
    }

    /**
     * Replaces the given node with a fresh one. Only to be called in case of websocket-error
     * 1001 or higher.
     * @param ws
     */
    replace(ws: IConnection): IConnection | undefined {
        this.nodes.gotError(ws.getURL());
        if (this.replied === 0) {
            return this.reserve.pop();
        }
        return undefined;
    }

    /**
     * Indicates that this node has successfully finished its job.
     * @param ws
     */
    done(ws: IConnection): number {
        const delay = Date.now() - this.start;
        if (this.replied === 0) {
            this.first = delay;
        }
        this.nodes.done(ws.getURL(), this.replied, delay / this.first);
        return this.replied++;
    }
}
