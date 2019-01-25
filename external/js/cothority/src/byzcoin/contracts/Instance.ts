import {InstanceID} from "../ClientTransaction";
import {Proof} from "../Proof";
import ByzCoinRPC from "../byzcoin-rpc";
import {Coin, CoinInstance} from "./CoinInstance";

export interface Instance {
    // These fields are available for every instance.
    iid: InstanceID;
    darcID: InstanceID;
    contractID: string;
    data: Buffer;
    bc: ByzCoinRPC;

    // Convert the instance
    toObject(): any;
}

export class BasicInstance implements Instance {
    public iid: InstanceID;
    public darcID: InstanceID;
    public data: Buffer;

    constructor(public bc: ByzCoinRPC, public contractID: string, p: Proof | { [k: string]: any }) {
        if (p) {
            if (p instanceof Proof) {
                if (p.matchContract(contractID)){
                    //this.iid = p.requestedIID;
                    this.darcID = p.darcID;
                    this.data = p.value;
                }
            } else {
                this.data = Buffer.from(p.data);
                this.iid = InstanceID.fromObject(p.iid);
                this.darcID = InstanceID.fromObject(p.darcID);
            }
        }
    }

    toObject(): any {
        return {
            iid: this.iid.toObject(),
            darcID: this.darcID.toObject(),
            data: this.data,
            contractID: this.contractID,
        }
    }
}