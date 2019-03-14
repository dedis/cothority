import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../../protobuf";
import ClientTransaction from "../client-transaction";

export default class TxResult extends Message<TxResult> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.TxResult", TxResult, ClientTransaction);
    }

    readonly clientTransaction: ClientTransaction;
    readonly accepted: boolean;

    constructor(props?: Properties<TxResult>) {
        super(props);

        /* Protobuf aliases */

        Object.defineProperty(this, "clienttransaction", {
            get(): ClientTransaction {
                return this.clientTransaction;
            },
            set(value: ClientTransaction) {
                this.clientTransaction = value;
            },
        });
    }
}

TxResult.register();
