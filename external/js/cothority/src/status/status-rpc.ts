import { Roster, ServerIdentity } from "../network/roster";
import { Connection, WebSocketConnection } from "../network/connection";
import { Message } from "protobufjs";
import { registerMessage } from "../protobuf";

class StatusRequest extends Message<StatusRequest> {}

class Status extends Message<Status> {
    private field: { [k: string]: string };

    getValue(k: string): string {
        return this.field[k];
    }

    toString(): string {
        return Object.keys(this.field).sort().map(k => `${k}: ${this.field[k]}`).join('\n');
    }
}

class StatusResponse extends Message<StatusResponse> {
    private status: { [k: string]: Status };
    private serveridentity: ServerIdentity;

    getStatus(key: string): Status {
        return this.status[key];
    }

    get serverIdentity(): ServerIdentity {
        return this.serveridentity;
    }

    toString(): string {
        return Object.keys(this.status).sort().map(k => `[${k}]\n${this.status[k].toString()}`).join('\n\n');
    }
}

registerMessage('Request', StatusRequest);
registerMessage('Response', StatusResponse);
registerMessage('Status', Status);

/**
 * RPC to talk with the status service of the conodes
 */
export default class SkipchainRPC {
    private conn: Connection;

    constructor(roster: Roster) {
        const addr = roster.list[0].address;

        this.conn = new WebSocketConnection(addr, 'Status');
    }

    async getStatus(): Promise<StatusResponse> {
        const buf = await this.conn.send(new StatusRequest());

        return StatusResponse.decode(buf);
    }
}
