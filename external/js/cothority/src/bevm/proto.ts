import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../protobuf";

export class CallRequest extends Message<CallRequest> {
    static register() {
        registerMessage("bevm.CallRequest", CallRequest);
    }

    readonly byzcoinId: Buffer;
    readonly serverConfig: string;
    readonly bevmInstanceId: Buffer;
    readonly accountAddress: Buffer;
    readonly contractAddress: Buffer;
    readonly callData: Buffer;

    constructor(props?: Properties<CallRequest>) {
        super(props);

        /* Protobuf aliases */
        Object.defineProperty(this, "byzcoinid", {
            get(): Buffer {
                return this.byzcoinId;
            },
            set(value: Buffer) {
                this.blockId = value;
            },
        });

        Object.defineProperty(this, "serverconfig", {
            get(): string {
                return this.serverConfig;
            },
            set(value: string) {
                this.serverConfig = value;
            },
        });

        Object.defineProperty(this, "bevminstanceid", {
            get(): Buffer {
                return this.bevmInstanceId;
            },
            set(value: Buffer) {
                this.bevmInstanceId = value;
            },
        });

        Object.defineProperty(this, "accountaddress", {
            get(): Buffer {
                return this.accountAddress;
            },
            set(value: Buffer) {
                this.accountAddress = value;
            },
        });

        Object.defineProperty(this, "contractaddress", {
            get(): Buffer {
                return this.contractAddress;
            },
            set(value: Buffer) {
                this.contractAddress = value;
            },
        });

        Object.defineProperty(this, "calldata", {
            get(): Buffer {
                return this.callData;
            },
            set(value: Buffer) {
                this.callData = value;
            },
        });
    }
}

export class CallResponse extends Message<CallResponse> {
    static register() {
        registerMessage("bevm.CallResponse", CallResponse);
    }

    readonly result: Buffer;
}

CallRequest.register();
CallResponse.register();
