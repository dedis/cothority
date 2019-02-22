import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../../protobuf";
import TxResult from "./tx-result";

/**
 * ByzCoin block payload
 */
export default class DataBody extends Message<DataBody> {
    readonly txResults: TxResult[];

    constructor(props?: Properties<DataBody>) {
        super(props);
    }

    /* Protobuf fields */

    private get txresults(): TxResult[] {
        return this.txResults;
    }

    private set txresults(value: TxResult[]) {
        // @ts-ignore
        this.txResults = value;
    }
}

registerMessage("byzcoin.DataBody", DataBody);
