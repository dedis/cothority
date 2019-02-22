import { Message } from "protobufjs/light";
import { ServerIdentity } from "../network/proto";
import { registerMessage } from "../protobuf";

/**
 * Status request message
 */
export class StatusRequest extends Message<StatusRequest> {}

/**
 * Status of a service
 */
export class Status extends Message<Status> {
    readonly field: { [k: string]: string };

    /**
     * Get the value of a field
     * @param field The name of the field
     * @returns the value or undefined
     */
    getValue(k: string): string {
        return this.field[k];
    }

    /**
     * Get a string representation of this status
     * @returns a string
     */
    toString(): string {
        return Object.keys(this.field).sort().map((k) => `${k}: ${this.field[k]}`).join("\n");
    }
}

/**
 * Status response message
 */
export class StatusResponse extends Message<StatusResponse> {
    readonly status: { [k: string]: Status };
    readonly serveridentity: ServerIdentity;

    /**
     * Get the status of a service
     * @param key The name of the service
     * @returns the status
     */
    getStatus(key: string): Status {
        return this.status[key];
    }

    /**
     * Get the server identity of the requested conode
     * @returns the server identity
     */
    get serverIdentity(): ServerIdentity {
        return this.serveridentity;
    }

    /**
     * Get a string representation of all the statuses
     * @returns a string
     */
    toString(): string {
        return Object.keys(this.status).sort().map((k) => `[${k}]\n${this.status[k].toString()}`).join("\n\n");
    }
}

registerMessage("Request", StatusRequest);
registerMessage("Response", StatusResponse);
registerMessage("Status", Status);
