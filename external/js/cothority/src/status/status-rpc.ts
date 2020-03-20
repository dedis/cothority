import { WebSocketConnection } from "../network";
import { Roster } from "../network/proto";
import { StatusRequest, StatusResponse } from "./proto";

/**
 * RPC to talk with the status service of the conodes
 */
export default class StatusRPC {
    static serviceName = "Status";

    private conn: WebSocketConnection[];
    private timeout: number;

    constructor(roster: Roster) {
        this.timeout = 10 * 1000;
        this.conn = roster.list
            .map((srvid) => new WebSocketConnection(srvid.getWebSocketAddress(), StatusRPC.serviceName));
    }

    /**
     * Set a new timeout value for the next requests
     * @param value Timeout in ms
     */
    setTimeout(value: number): void {
        this.timeout = value;
    }

    /**
     * Fetch the status of the server at the given index
     * @param index Index of the server identity
     * @returns a promise that resolves with the status response
     */
    async getStatus(index: number = 0): Promise<StatusResponse> {
        if (index >= this.conn.length || index < 0) {
            throw new Error("Index out of bound for the roster");
        }

        this.conn[index].setTimeout(this.timeout);

        return this.conn[index].send(new StatusRequest(), StatusResponse);
    }
}
