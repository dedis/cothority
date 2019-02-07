import { IConnection, WebSocketConnection } from "../network/connection";
import { Roster } from "../network/proto";
import { StatusRequest, StatusResponse } from "./proto";

/**
 * RPC to talk with the status service of the conodes
 */
export default class StatusRPC {
    static serviceName = "Status";

    private conn: IConnection[];

    constructor(roster: Roster) {
        this.conn = roster.list
            .map((srvid) => new WebSocketConnection(srvid.getWebSocketAddress(), StatusRPC.serviceName));
    }

    async getStatus(index: number = 0): Promise<StatusResponse> {
        if (index >= this.conn.length) {
            throw new Error("Index out of bound for the roster");
        }

        return this.conn[index].send(new StatusRequest(), StatusResponse);
    }
}
