import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../../protobuf";
import TxResult from "./tx-result";

import "./tx-result"; // messages registration

/**
 * ByzCoin block payload
 */
export default class DataBody extends Message<DataBody> {
    readonly txResults: TxResult[];

    constructor(props?: Properties<DataBody>) {
        super(props);

        this.txResults = this.txResults || [];

        /* Protobuf aliases */

        Object.defineProperty(this, "txresults", {
            get(): TxResult[] {
                return this.txResults;
            },
            set(value: TxResult[]) {
                this.txResults = value;
            },
        });
    }
}

registerMessage("byzcoin.DataBody", DataBody);
