import { Message, Properties } from "protobufjs/light";
import { ServerIdentity } from "../network/proto";
import { registerMessage } from "../protobuf";

/**
 * Status request message
 */
export class StatusRequest extends Message<StatusRequest> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Request", StatusRequest);
    }
}

/**
 * Status of a service
 */
export class Status extends Message<Status> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Status", Status);
    }

    readonly field: { [k: string]: string };

    constructor(props?: Properties<Status>) {
        super(props);

        this.field = this.field || {};
    }

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
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Response", StatusResponse, Status, ServerIdentity);
    }

    readonly status: { [k: string]: Status };
    readonly serverIdentity: ServerIdentity;

    constructor(props?: Properties<StatusResponse>) {
        super(props);

        this.status = this.status || {};

        /* Protobuf aliases */

        Object.defineProperty(this, "serveridentity", {
            get(): ServerIdentity {
                return this.serverIdentity;
            },
            set(value: ServerIdentity) {
                this.serverIdentity = value;
            },
        });
    }

    /**
     * Get the status of a service
     * @param key The name of the service
     * @returns the status
     */
    getStatus(key: string): Status {
        return this.status[key];
    }

    /**
     * Get a string representation of all the statuses
     * @returns a string
     */
    toString(): string {
        return Object.keys(this.status).sort().map((k) => `[${k}]\n${this.status[k].toString()}`).join("\n\n");
    }
}

StatusRequest.register();
StatusResponse.register();
Status.register();
