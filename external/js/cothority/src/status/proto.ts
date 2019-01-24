import { ServerIdentity } from "../network/proto";
import { Message } from "protobufjs";
import { registerMessage } from "../protobuf";

export class StatusRequest extends Message<StatusRequest> {}

export class Status extends Message<Status> {
    readonly field: { [k: string]: string };

    getValue(k: string): string {
        return this.field[k];
    }

    toString(): string {
        return Object.keys(this.field).sort().map(k => `${k}: ${this.field[k]}`).join('\n');
    }
}

export class StatusResponse extends Message<StatusResponse> {
    readonly status: { [k: string]: Status };
    readonly serveridentity: ServerIdentity;

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
