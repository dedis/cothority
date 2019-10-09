import shuffle from "shuffle-array";
import Log from "../log";
import { WebSocketConnection } from "./connection";
import { Roster } from "./proto";

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
    // is > 0, it will be enforced.
    static parallel: number = 0;
    // put in the 'reserve'.
    static slowThreshold: number = 10;

    // Holds the fastest connections for 'parallel' nodes. They are sorted from the fastest
    // This can be set from the outside to enforce a number of parallel requests. If it
    private readonly addresses: string[];

    // the threshold for the ratio of response_node / response_fastest where the node is
    // to the slowest node.
    private readonly active: Node[] = [];
    // is the one that was not used for the longest time.
    private readonly reserve: Node[];

    // Current number of nodes in the active queue.
    private _parallel: number;

    constructor(r: Roster, parallel: number = 3) {
        this.addresses = r.list.map((conode) => conode.getWebSocketAddress());
        shuffle(this.addresses);
        // Initialize the pool of connections
        this.reserve = this.addresses.map((addr) => new Node(addr));
        this.parallel = parallel;
    }

    /**
     * Returns the number of nodes currently in the active queue.
     */
    get parallel(): number {
        return this._parallel;
    }

    // Pool of available connections that are not in the active connections. The first node

    /**
     * Resets the number of active connections
     * @param p active connections
     */
    set parallel(p: number) {
        if (Nodes.parallel > 0) {
            p = Nodes.parallel;
        }
        if (p > this.addresses.length) {
            this._parallel = this.addresses.length;
        } else if (p > 0) {
            this._parallel = p;
        }
        while (this._parallel > this.active.length) {
            this.active.push(this.reserve.shift());
        }
        while (this._parallel < this.active.length) {
            this.reserve.push(this.active.pop());
        }
    }

    /**
     * Creates a new NodeList for one message.
     * @param service
     */
    newList(service: string): NodeList {
        if (Nodes.parallel > 0) {
            this.parallel = Nodes.parallel;
        }
        return new NodeList(this, service, this.active.slice(), this.reserve.slice());
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
                this.swapActive(index, rang);
            }
        }
    }

    /**
     * Sets the timeout on all nodes.
     * @param t
     */
    setTimeout(t: number) {
        this.active.concat(this.reserve).forEach((n) => n.setTimeout(t));
    }

    /**
     * Replaces the given node from the active queue with the first node from
     * the reserve queue.
     * @param index
     */
    private replaceActive(index: number) {
        if (index >= 0) {
            this.reserve.push(this.active.splice(index, 1)[0]);
            this.active.push(this.reserve.shift());
        }
    }

    /**
     * Swaps two nodes in the active queue.
     */
    private swapActive(a: number, b: number) {
        if (a >= 0 && b >= 0 &&
            a < this.active.length && b < this.active.length) {
            [this.active[a], this.active[b]] =
                [this.active[b], this.active[a]];
        } else {
            Log.error("asked to swap", a, b, this.active.length);
        }
    }

    /**
     * Gets the index of a given address.
     * @param address
     */
    private index(address: string): number {
        return this.active.findIndex((c) => {
            return c.address === address;
        });
    }
}

/**
 * A Node holds one WebSocketConnection per service.
 */
export class Node {
    private services: Map<string, WebSocketConnection> = new Map<string, WebSocketConnection>();

    constructor(readonly address: string) {
    }

    /**
     * Returns a WebSocketConnection for a given service. If the
     * connection doesn't exist yet, it will be created.
     * @param name
     */
    getService(name: string): WebSocketConnection {
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
    readonly active: WebSocketConnection[];
    private reserve: WebSocketConnection[];
    private readonly start: number;
    private first: number = 0;
    private replied: number = 0;

    constructor(private nodes: Nodes, service: string, active: Node[], reserve: Node[]) {
        this.start = Date.now();
        this.active = active.map((a) => a.getService(service));
        this.reserve = reserve.map((r) => r.getService(service));
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
    replace(ws: WebSocketConnection): WebSocketConnection | undefined {
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
    done(ws: WebSocketConnection): number {
        const delay = Date.now() - this.start;
        if (this.replied === 0) {
            this.first = delay;
        }
        this.nodes.done(ws.getURL(), this.replied, delay / this.first);
        return this.replied++;
    }
}
