import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../protobuf";

export class DeployRequest extends Message<DeployRequest> {
    static register() {
        registerMessage("bevm.DeployRequest", DeployRequest);
    }

    readonly gasLimit: number;
    readonly gasPrice: number;
    readonly amount: number;
    readonly nonce: number;
    readonly bytecode: Buffer;
    readonly abi: string;
    readonly args: string[];

    constructor(props?: Properties<DeployRequest>) {
        super(props);

        /* Protobuf aliases */
        Object.defineProperty(this, "gaslimit", {
            get(): number {
                return this.gasLimit;
            },
            set(value: number) {
                this.gasLimit = value;
            },
        });

        Object.defineProperty(this, "gasprice", {
            get(): number {
                return this.gasPrice;
            },
            set(value: number) {
                this.gasPrice = value;
            },
        });
    }
}

export class TransactionRequest extends Message<TransactionRequest> {
    static register() {
        registerMessage("bevm.TransactionRequest", TransactionRequest);
    }

    readonly gasLimit: number;
    readonly gasPrice: number;
    readonly amount: number;
    readonly contractAddress: Buffer;
    readonly nonce: number;
    readonly abi: string;
    readonly method: string;
    readonly args: string[];

    constructor(props?: Properties<TransactionRequest>) {
        super(props);

        /* Protobuf aliases */
        Object.defineProperty(this, "gaslimit", {
            get(): number {
                return this.gasLimit;
            },
            set(value: number) {
                this.gasLimit = value;
            },
        });

        Object.defineProperty(this, "gasprice", {
            get(): number {
                return this.gasPrice;
            },
            set(value: number) {
                this.gasPrice = value;
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
    }
}

export class TransactionHashResponse extends Message<TransactionHashResponse> {
    static register() {
        registerMessage("bevm.TransactionHashResponse",
                        TransactionHashResponse);
    }

    readonly transaction: Buffer;
    readonly transactionHash: Buffer;

    constructor(props?: Properties<TransactionHashResponse>) {
        super(props);

        /* Protobuf aliases */
        Object.defineProperty(this, "transactionhash", {
            get(): Buffer {
                return this.transactionHash;
            },
            set(value: Buffer) {
                this.transactionHash = value;
            },
        });
    }
}

export class TransactionFinalizationRequest extends Message<TransactionFinalizationRequest> {
    static register() {
        registerMessage("bevm.TransactionFinalizationRequest",
                        TransactionFinalizationRequest);
    }

    readonly transaction: Buffer;
    readonly signature: Buffer;
}

export class TransactionResponse extends Message<TransactionResponse> {
    static register() {
        registerMessage("bevm.TransactionResponse", TransactionResponse);
    }

    readonly transaction: Buffer;
}

export class CallRequest extends Message<CallRequest> {
    static register() {
        registerMessage("bevm.CallRequest", CallRequest);
    }

    readonly blockId: Buffer;
    readonly serverConfig: string;
    readonly bevmInstanceId: Buffer;
    readonly accountAddress: Buffer;
    readonly contractAddress: Buffer;
    readonly abi: string;
    readonly method: string;
    readonly args: string[];

    constructor(props?: Properties<CallRequest>) {
        super(props);

        /* Protobuf aliases */
        Object.defineProperty(this, "blockid", {
            get(): Buffer {
                return this.blockId;
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
    }
}

export class CallResponse extends Message<CallResponse> {
    static register() {
        registerMessage("bevm.CallResponse", CallResponse);
    }

    readonly result: string;
}

DeployRequest.register();
TransactionRequest.register();
TransactionHashResponse.register();
TransactionFinalizationRequest.register();
TransactionResponse.register();
CallRequest.register();
CallResponse.register();
