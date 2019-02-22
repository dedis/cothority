import { Message } from "protobufjs/light";
import { registerMessage } from "../../protobuf";
import ClientTransaction from "../client-transaction";

export default class TxResult extends Message<TxResult> {
    readonly clienttransaction: ClientTransaction;
    readonly accepted: boolean;

    /**
     * Getter for the client transaction
     *
     * @returns the transaction
     */
    get clientTransaction(): ClientTransaction {
        return this.clienttransaction;
    }
}

registerMessage("byzcoin.TxResult", TxResult);
