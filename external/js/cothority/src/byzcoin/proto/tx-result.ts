import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../../protobuf";
import ClientTransaction from "../client-transaction";

import "../client-transaction"; // messages registration

export default class TxResult extends Message<TxResult> {
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

registerMessage("byzcoin.TxResult", TxResult);
